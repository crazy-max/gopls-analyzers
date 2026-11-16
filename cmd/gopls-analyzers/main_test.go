package main

import (
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/modernize"
)

func TestExclusionsFilterFixes(t *testing.T) {
	fset := token.NewFileSet()
	var files []*ast.File
	for _, name := range []string{"handwritten.go", "ignored.go"} {
		file, err := parser.ParseFile(fset, filepath.Join(t.TempDir(), name), "package fixture\nvar Value int\n", parser.ParseComments)
		if err != nil {
			t.Fatal(err)
		}
		files = append(files, file)
	}
	var diagnostics []analysis.Diagnostic
	pass := &analysis.Pass{
		Fset:  fset,
		Files: files,
		Report: func(diagnostic analysis.Diagnostic) {
			diagnostics = append(diagnostics, diagnostic)
		},
	}
	exclude := exclusions{pattern: regexp.MustCompile(`/ignored\.go$`)}
	filtered := exclude.filter(pass)
	original := analysis.Diagnostic{
		Pos: files[0].Pos(),
		SuggestedFixes: []analysis.SuggestedFix{
			{
				Message: "safe fix",
				TextEdits: []analysis.TextEdit{
					{Pos: files[0].Pos(), NewText: []byte("safe")},
				},
			},
			{
				Message: "cross-file fix",
				TextEdits: []analysis.TextEdit{
					{Pos: files[0].Pos(), NewText: []byte("safe")},
					{Pos: files[1].Pos(), NewText: []byte("excluded")},
				},
			},
		},
	}
	filtered.Report(original)
	filtered.Report(analysis.Diagnostic{Pos: files[1].Pos()})
	filtered.Report(analysis.Diagnostic{Message: "no position"})
	if len(diagnostics) != 2 {
		t.Fatalf("got %d diagnostics, want 2", len(diagnostics))
	}
	if fixes := diagnostics[0].SuggestedFixes; len(fixes) != 1 || fixes[0].Message != "safe fix" {
		t.Errorf("filtered fixes = %v, want only safe fix", fixes)
	}
	if len(original.SuggestedFixes) != 2 || original.SuggestedFixes[1].Message != "cross-file fix" {
		t.Error("filter mutated the original fixes")
	}
	if len(filtered.Files) != len(pass.Files) || pass.Report == nil {
		t.Error("filter removed files from analysis")
	}
}

func TestCLI(t *testing.T) {
	binary := filepath.Join(t.TempDir(), "gopls-analyzers")
	if runtime.GOOS == "windows" {
		binary += ".exe"
	}
	args := []string{"build", "-o", binary}
	if os.Getenv("GOCOVERDIR") != "" {
		// Collect coverage from the CLI subprocesses, including go vet's vettool.
		args = append(args, "-cover", "-covermode=atomic", "-race",
			"-coverpkg=github.com/crazy-max/gopls-analyzers/cmd/...,github.com/crazy-max/gopls-analyzers/internal/gopls/analysis/...")
	}
	if output, err := exec.CommandContext(t.Context(), "go", append(args, ".")...).CombinedOutput(); err != nil {
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
		if !names["exclude-generated"] {
			t.Error("missing boolean exclude-generated flag")
		}
		if !names["verbose"] {
			t.Error("missing boolean verbose flag")
		}
		if isBool, ok := names["exclude"]; !ok || isBool {
			t.Error("missing string exclude flag")
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
		files    map[string]string
	}{
		{
			name:   "verbose vet",
			flags:  []string{"-verbose", "-any"},
			source: "package fixture\nvar Value any\n",
			want: []string{
				"gopls-analyzers: selected analyzers (1):\n  - any\n",
				"gopls-analyzers: example.com/fixture: any: starting (files: 1)",
				"gopls-analyzers: example.com/fixture: any: completed in ",
			},
			absent: []string{"infertypeargs: starting"},
		},
		{
			name:   "verbose diagnostics",
			flags:  []string{"-verbose", "-any"},
			source: "package fixture\nvar Value interface{}\n",
			want: []string{
				"gopls-analyzers: example.com/fixture: any: completed in ",
				"interface{} can be replaced by any",
			},
			wantErr: true,
		},
		{
			name:   "verbose direct",
			flags:  []string{"-verbose", "-any"},
			source: "package fixture\nvar Value any\n",
			want: []string{
				"gopls-analyzers: selected analyzers (1):\n  - any\n",
				"gopls-analyzers: example.com/fixture: any: starting (files: 1)",
				"gopls-analyzers: example.com/fixture: any: completed in ",
			},
			direct: true,
		},
		{
			name:   "verbose selected analyzers",
			flags:  []string{"-verbose", "-infertypeargs", "-any", "-yield=false"},
			source: "package fixture\nvar Value any\n",
			want:   []string{"gopls-analyzers: selected analyzers (2):\n  - any\n  - infertypeargs\n"},
		},
		{
			name:   "verbose negative analyzer selection",
			flags:  []string{"-verbose", "-any=false"},
			source: "package fixture\nvar Value any\n",
			want:   []string{fmt.Sprintf("gopls-analyzers: selected analyzers (%d):\n  - atomictypes\n", 9+len(modernize.Suite))},
			absent: []string{"any: starting", "  - any\n"},
		},
		{
			name:   "verbose default analyzers",
			flags:  []string{"-verbose", "-exclude-generated"},
			source: "package fixture\nvar Value any\n",
			want: []string{
				fmt.Sprintf("gopls-analyzers: selected analyzers (%d):\n  - any\n  - atomictypes\n", 10+len(modernize.Suite)),
				"gopls-analyzers: example.com/fixture: any: completed in ",
				"gopls-analyzers: example.com/fixture: infertypeargs: completed in ",
			},
		},
		{
			name:   "verbose disabled",
			flags:  []string{"-verbose=false"},
			source: "package fixture\nvar Value any\n",
			absent: []string{"gopls-analyzers:"},
		},
		{
			name:   "quiet by default",
			source: "package fixture\nvar Value any\n",
			absent: []string{"gopls-analyzers:"},
		},
		{
			name:    "generated diagnostics by default",
			flags:   []string{"-any"},
			source:  "// Code generated by fixture. DO NOT EDIT.\npackage fixture\nvar Value interface{}\n",
			want:    []string{"interface{} can be replaced by any"},
			wantErr: true,
		},
		{
			name:   "exclude generated vet",
			flags:  []string{"-exclude-generated"},
			source: "// Code generated by fixture. DO NOT EDIT.\npackage fixture\nvar Value interface{}\n",
		},
		{
			name:   "exclude generated fix",
			flags:  []string{"-any", "-exclude-generated", "-fix"},
			source: "// Code generated by fixture. DO NOT EDIT.\npackage fixture\nvar Value interface{}\n",
			direct: true,
			fixed:  "// Code generated by fixture. DO NOT EDIT.\npackage fixture\nvar Value interface{}\n",
		},
		{
			name:   "exclude path vet",
			flags:  []string{"-exclude=(^|/)fixture\\.go$"},
			source: "package fixture\nvar Value interface{}\n",
		},
		{
			name:   "exclude path fix",
			flags:  []string{"-any", "-exclude=fixture\\.go$", "-fix"},
			source: "package fixture\nvar Value interface{}\n",
			direct: true,
			fixed:  "package fixture\nvar Value interface{}\n",
		},
		{
			name:    "nonmatching path",
			flags:   []string{"-any", "-exclude=other\\.go$"},
			source:  "package fixture\nvar Value interface{}\n",
			want:    []string{"interface{} can be replaced by any"},
			wantErr: true,
		},
		{
			name:    "invalid exclude pattern vet",
			flags:   []string{"-exclude=["},
			source:  "package fixture\n",
			want:    []string{"error parsing regexp"},
			wantErr: true,
		},
		{
			name:    "invalid exclude pattern direct",
			flags:   []string{"-exclude=["},
			source:  "package fixture\n",
			want:    []string{"error parsing regexp"},
			wantErr: true,
			direct:  true,
		},
		{
			name:   "combined exclusions retain handwritten diagnostics",
			flags:  []string{"-any", "-exclude-generated", "-exclude=ignored\\.go$"},
			source: "package fixture\nvar Value interface{}\nvar FromGenerated = Generated\n",
			files: map[string]string{
				"generated.go": "// Code generated by fixture. DO NOT EDIT.\npackage fixture\nvar Generated interface{}\n",
				"ignored.go":   "package fixture\nvar Ignored interface{}\n",
			},
			want:    []string{"fixture.go:2", "interface{} can be replaced by any"},
			absent:  []string{"generated.go:", "ignored.go:"},
			wantErr: true,
		},
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
			name:    "simplify composite literals",
			flags:   []string{"-simplifycompositelit"},
			source:  "package fixture\ntype Point struct { X int }\nvar Points = []*Point{&Point{X: 1}}\nvar Lookup = map[Point]Point{Point{X: 1}: Point{X: 2}}\nvar Array = [1]Point{Point{X: 3}}\n",
			want:    []string{"redundant type in slice literal", "redundant type in map literal", "redundant type in array literal"},
			wantErr: true,
		},
		{
			name:   "composite literal fix",
			flags:  []string{"-simplifycompositelit", "-fix"},
			source: "package fixture\n\nvar Values = [][]int{[]int{1}}\n",
			direct: true,
			fixed:  "package fixture\n\nvar Values = [][]int{{1}}\n",
		},
		{
			name:   "generated composite literals",
			flags:  []string{"-simplifycompositelit"},
			source: "// Code generated by fixture. DO NOT EDIT.\npackage fixture\nvar Values = [][]int{[]int{1}}\n",
		},
		{
			name:   "slice fix",
			flags:  []string{"-simplifyslice", "-fix"},
			source: "package fixture\n\nfunc Slice(s []int) []int { return s[1:len(s)] }\n",
			direct: true,
			fixed:  "package fixture\n\nfunc Slice(s []int) []int { return s[1:] }\n",
		},
		{
			name:   "preserve slice bounds",
			flags:  []string{"-simplifyslice"},
			source: "package fixture\nfunc Slice(s, other []int) []int { return s[:len(other)] }\nfunc Full(s []int) []int { return s[:len(s):len(s)] }\n",
		},
		{
			name:    "map range diagnostics",
			flags:   []string{"-maprange"},
			source:  "package fixture\nimport \"maps\"\nfunc Keys(m map[string]int) { for k := range maps.Keys(m) { _ = k } }\nfunc Values(m map[string]int) { for v := range maps.Values(m) { _ = v } }\n",
			want:    []string{"unnecessary and inefficient call of maps.Keys", "unnecessary and inefficient call of maps.Values"},
			wantErr: true,
		},
		{
			name:   "map values fix",
			flags:  []string{"-maprange", "-fix"},
			source: "package fixture\n\nimport \"maps\"\n\nfunc Values(m map[string]int) {\n\tfor v := range maps.Values(m) {\n\t\t_ = v\n\t}\n}\n",
			direct: true,
			fixed:  "package fixture\n\nfunc Values(m map[string]int) {\n\tfor _, v := range m {\n\t\t_ = v\n\t}\n}\n",
		},
		{
			name:   "preserve map iterators outside range",
			flags:  []string{"-maprange"},
			source: "package fixture\nimport \"maps\"\nvar Keys = maps.Keys(map[int]int{1: 2})\nvar Values = maps.Values(map[int]int{1: 2})\n",
		},
		{
			name:    "redeclared variable diagnostics",
			flags:   []string{"-nonewvars", "-json"},
			source:  "package fixture\nfunc Value() int { n := 1; n := 2; return n }\n",
			want:    []string{"\"nonewvars\"", "Change ':=' to '='"},
			wantErr: true,
			direct:  true,
		},
		{
			name:    "unexpected return value diagnostics",
			flags:   []string{"-noresultvalues", "-json"},
			source:  "package fixture\nfunc Value() { return 1 }\n",
			want:    []string{"\"noresultvalues\"", "Delete return values"},
			wantErr: true,
			direct:  true,
		},
		{
			name:    "missing embed import",
			flags:   []string{"-embed", "-json"},
			source:  "package fixture\n//go:embed asset.txt\nvar Asset string\n",
			files:   map[string]string{"asset.txt": "fixture\n"},
			want:    []string{"\"embed\"", "Add missing"},
			wantErr: true,
			direct:  true,
		},
		{
			name:   "valid embed types",
			flags:  []string{"-embed"},
			source: "package fixture\nimport \"embed\"\n//go:embed asset.txt\nvar Text string\n//go:embed asset.txt\nvar Bytes []byte\n//go:embed asset.txt\nvar Files embed.FS\n",
			files:  map[string]string{"asset.txt": "fixture\n"},
		},
		{
			name:    "yield after stopping",
			flags:   []string{"-yield"},
			source:  "package fixture\nimport \"iter\"\nfunc Broken() iter.Seq[int] { return func(yield func(int) bool) { yield(1); yield(2) } }\nfunc Loop() iter.Seq[int] { return func(yield func(int) bool) { for i := range 3 { yield(i) } } }\n",
			want:    []string{"yield may be called again"},
			wantErr: true,
		},
		{
			name:  "yield respects stopping",
			flags: []string{"-yield"},
			source: `package fixture
import "iter"
func Single() iter.Seq[int] {
	return func(yield func(int) bool) { yield(1) }
}
func Loop() iter.Seq[int] {
	return func(yield func(int) bool) {
		for i := range 3 {
			if !yield(i) { return }
		}
	}
}
func Conditional() iter.Seq[int] {
	return func(yield func(int) bool) { if yield(1) { yield(2) } }
}
func ShortCircuit() iter.Seq[int] {
	return func(yield func(int) bool) { _ = yield(1) && yield(2) && yield(3) }
}
func Saved() iter.Seq[int] {
	return func(yield func(int) bool) {
		ok := yield(1)
		if ok { ok = yield(2) }
		if !ok { return }
		yield(3)
	}
}
func Pairs() iter.Seq2[int, string] {
	return func(yield func(int, string) bool) {
		if !yield(1, "one") { return }
		yield(2, "two")
	}
}
`,
		},
		{
			name:  "required parameter signatures",
			flags: []string{"-unusedparams"},
			source: `package fixture
type contract interface { method(value int) int }
type impl struct{}
func (impl) method(value int) int { return 1 }
var _ contract = impl{}
func callback(value int) int { return 1 }
var Callback = callback
func Exported(value int) int { return 1 }
func used(value int) int { return value }
func blank(_ int) int { return 1 }
`,
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
			files := map[string]string{
				"go.mod":     "module example.com/fixture\n\ngo 1.27.0\n",
				"fixture.go": tc.source,
			}
			for name, content := range tc.files {
				files[name] = content
			}
			for name, content := range files {
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
