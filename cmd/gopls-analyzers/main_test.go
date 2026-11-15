package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"golang.org/x/tools/go/analysis/passes/modernize"
)

func TestCLI(t *testing.T) {
	binary := filepath.Join(t.TempDir(), "gopls-analyzers")
	if runtime.GOOS == "windows" {
		binary += ".exe"
	}
	if output, err := exec.CommandContext(t.Context(), "go", "build", "-o", binary, ".").CombinedOutput(); err != nil {
		t.Fatalf("build runner: %v\n%s", err, output)
	}

	t.Run("flags", func(t *testing.T) {
		output, err := exec.CommandContext(t.Context(), binary, "-flags").CombinedOutput()
		if err != nil {
			t.Fatalf("list flags: %v\n%s", err, output)
		}
		var flags []struct {
			Name string
			Bool bool
		}
		if err := json.Unmarshal(output, &flags); err != nil {
			t.Fatal(err)
		}
		names := make(map[string]bool)
		for _, flag := range flags {
			names[flag.Name] = flag.Bool
		}
		for _, name := range []string{
			"embed", "fillreturns", "infertypeargs", "maprange",
			"nonewvars", "noresultvalues", "simplifycompositelit", "simplifyslice",
			"unusedparams", "yield",
		} {
			if !names[name] {
				t.Errorf("missing analyzer flag %q", name)
			}
		}
		for _, analyzer := range modernize.Suite {
			if !names[analyzer.Name] {
				t.Errorf("missing modernize analyzer flag %q", analyzer.Name)
			}
		}
	})

	t.Run("help", func(t *testing.T) {
		output, err := exec.CommandContext(t.Context(), binary, "help", "unusedparams").CombinedOutput()
		if err != nil || !strings.Contains(string(output), "unusedparams") {
			t.Fatalf("analyzer help: %v\n%s", err, output)
		}
	})

	const source = `package fixture

func Identity[T any](v T) T { return v }

var Value = Identity[int](1)

func Slice(s []int) []int { return s[:len(s)] }
`
	for _, tc := range []struct {
		name     string
		flags    []string
		source   string
		want     []string
		absent   []string
		wantErr  bool
		direct   bool
		wantExit int
		fixed    string
	}{
		{
			name:    "modernize by default",
			source:  "package fixture\nvar Value interface{}\n",
			want:    []string{"interface{} can be replaced by any"},
			wantErr: true,
		},
		{
			name:    "select modernize analyzer",
			flags:   []string{"-any"},
			source:  source + "\nvar Other interface{}\n",
			want:    []string{"interface{} can be replaced by any"},
			absent:  []string{"unnecessary type arguments", "unneeded: len(s)"},
			wantErr: true,
		},
		{
			name:    "select gopls and modernize analyzers",
			flags:   []string{"-infertypeargs", "-any"},
			source:  source + "\nvar Other interface{}\n",
			want:    []string{"unnecessary type arguments", "interface{} can be replaced by any"},
			absent:  []string{"unneeded: len(s)"},
			wantErr: true,
		},
		{
			name:   "disable modernize analyzer",
			flags:  []string{"-any=false"},
			source: "package fixture\nvar Value interface{}\n",
		},
		{
			name:   "modernize fix",
			flags:  []string{"-any", "-fix"},
			source: "package fixture\n\nvar Value interface{}\n",
			direct: true,
			fixed:  "package fixture\n\nvar Value any\n",
		},
		{
			name:     "direct diagnostics",
			flags:    []string{"-infertypeargs"},
			source:   source,
			want:     []string{"unnecessary type arguments"},
			absent:   []string{"unneeded: len(s)"},
			wantErr:  true,
			direct:   true,
			wantExit: 3,
		},
		{
			name:   "direct fix",
			flags:  []string{"-infertypeargs", "-fix"},
			source: source,
			direct: true,
			fixed:  strings.ReplaceAll(source, "Identity[int](1)", "Identity(1)"),
		},
		{
			name:    "fill incomplete return",
			flags:   []string{"-fillreturns", "-json"},
			source:  "package fixture\nfunc Returns() int { return }\n",
			want:    []string{"\"fillreturns\""},
			direct:  true,
			wantErr: true,
		},
		{
			name:    "all analyzers",
			source:  source,
			want:    []string{"unnecessary type arguments", "unneeded: len(s)"},
			wantErr: true,
		},
		{
			name:    "select one",
			flags:   []string{"-infertypeargs"},
			source:  source,
			want:    []string{"unnecessary type arguments"},
			absent:  []string{"unneeded: len(s)"},
			wantErr: true,
		},
		{
			name:    "select multiple",
			flags:   []string{"-infertypeargs", "-simplifyslice"},
			source:  source,
			want:    []string{"unnecessary type arguments", "unneeded: len(s)"},
			wantErr: true,
		},
		{
			name:    "disable one",
			flags:   []string{"-infertypeargs=false"},
			source:  source,
			want:    []string{"unneeded: len(s)"},
			absent:  []string{"unnecessary type arguments"},
			wantErr: true,
		},
		{
			name:   "disable both",
			flags:  []string{"-infertypeargs=false", "-simplifyslice=false"},
			source: source,
		},
		{
			name:   "clean package",
			source: "package fixture\nfunc Identity[T any](v T) T { return v }\nvar Value = Identity(1)\n",
		},
		{
			name:    "unused parameter",
			source:  "package fixture\nfunc unused(value int) int { return 1 }\n",
			want:    []string{"unused parameter: value"},
			wantErr: true,
		},
		{
			name:    "unknown analyzer",
			flags:   []string{"-notananalyzer"},
			source:  "package fixture\n",
			want:    []string{"flag provided but not defined"},
			wantErr: true,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			for name, content := range map[string]string{
				"go.mod":     "module example.com/fixture\n\ngo 1.27.0\n",
				"fixture.go": tc.source,
			} {
				if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			args := append([]string{"vet", "-vettool=" + binary}, tc.flags...)
			cmd := exec.CommandContext(t.Context(), "go", append(args, "./...")...)
			if tc.direct {
				cmd = exec.CommandContext(t.Context(), binary, append(tc.flags, "./...")...)
			}
			cmd.Dir = dir
			cmd.Env = append(os.Environ(), "GOWORK=off", "GOPROXY=off")
			output, err := cmd.CombinedOutput()
			if (err != nil) != tc.wantErr {
				t.Fatalf("error = %v, want error = %v\n%s", err, tc.wantErr, output)
			}
			if tc.wantExit != 0 {
				if exitErr, ok := err.(*exec.ExitError); !ok || exitErr.ExitCode() != tc.wantExit {
					t.Errorf("error = %v, want exit code %d", err, tc.wantExit)
				}
			}
			if tc.fixed != "" {
				fixed, err := os.ReadFile(filepath.Join(dir, "fixture.go"))
				if err != nil {
					t.Fatal(err)
				}
				if string(fixed) != tc.fixed {
					t.Errorf("fixed source = %q, want %q", fixed, tc.fixed)
				}
			}
			for _, want := range tc.want {
				if !strings.Contains(string(output), want) {
					t.Errorf("missing %q in output:\n%s", want, output)
				}
			}
			for _, absent := range tc.absent {
				if strings.Contains(string(output), absent) {
					t.Errorf("unexpected %q in output:\n%s", absent, output)
				}
			}
		})
	}
}
