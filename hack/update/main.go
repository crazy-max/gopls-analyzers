// Command update copies the selected upstream analyzers and their internal
// dependencies. Run it from the repository root with go run ./hack/update.
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"go/format"
	"go/parser"
	"go/token"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
)

const module = "github.com/crazy-max/gopls-analyzers"

type download struct {
	Path, Version, Dir, GoMod, Sum string
}

func main() {
	log.SetFlags(0)
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() (retErr error) {
	pin, err := os.ReadFile("hack/gopls.version")
	if err != nil {
		return err
	}
	gopls, err := fetch("golang.org/x/tools/gopls@" + strings.TrimSpace(string(pin)))
	if err != nil {
		return err
	}
	var mod struct {
		Go      string
		Require []struct{ Path, Version string }
	}
	if err := goJSON(&mod, "mod", "edit", "-json", gopls.GoMod); err != nil {
		return err
	}
	var tools download
	for _, req := range mod.Require {
		if req.Path == "golang.org/x/tools" {
			tools, err = fetch(req.Path + "@" + req.Version)
			if err != nil {
				return err
			}
		}
	}
	if tools.Dir == "" {
		return fmt.Errorf("gopls has no x/tools requirement")
	}
	stage, err := os.MkdirTemp("", "gopls-analyzers-upstream-")
	if err != nil {
		return err
	}
	defer func() {
		if retErr != nil {
			log.Printf("update failed; staged sources retained at %s", stage)
			return
		}
		if err := os.RemoveAll(stage); err != nil {
			log.Printf("remove update staging directory: %v", err)
		}
	}()
	sources := []struct{ prefix, local, dir string }{
		{"golang.org/x/tools/gopls/internal/", "internal/gopls/", filepath.Join(gopls.Dir, "internal")},
		{"golang.org/x/tools/internal/", "internal/xtools/", filepath.Join(tools.Dir, "internal")},
	}
	entry, err := parser.ParseFile(token.NewFileSet(), "cmd/gopls-analyzers/main.go", nil, parser.ImportsOnly)
	if err != nil {
		return err
	}
	var queue []string
	for _, imp := range entry.Imports {
		path, _ := strconv.Unquote(imp.Path.Value)
		for _, src := range sources {
			if suffix, ok := strings.CutPrefix(path, module+"/"+src.local); ok {
				queue = append(queue, src.prefix+suffix)
			}
		}
	}
	if len(queue) == 0 {
		return fmt.Errorf("no copied upstream packages found in runner")
	}
	seen := make(map[string]bool)
	for len(queue) > 0 {
		path := queue[0]
		queue = queue[1:]
		if seen[path] {
			continue
		}
		seen[path] = true
		for _, src := range sources {
			suffix, ok := strings.CutPrefix(path, src.prefix)
			if !ok {
				continue
			}
			from := filepath.Join(src.dir, filepath.FromSlash(suffix))
			to := filepath.Join(stage, src.local, filepath.FromSlash(suffix))
			entries, err := os.ReadDir(from)
			if err != nil {
				return err
			}
			for _, entry := range entries {
				name := entry.Name()
				if entry.IsDir() || strings.HasSuffix(name, "_test.go") {
					continue
				}
				data, err := os.ReadFile(filepath.Join(from, name))
				if err != nil {
					return err
				}
				if strings.HasSuffix(name, ".go") {
					fset := token.NewFileSet()
					file, err := parser.ParseFile(fset, name, data, parser.ParseComments)
					if err != nil {
						return err
					}
					// Files in this directory (such as doc.go) are copied below.
					// Reject patterns that would require recursive asset copying.
					for _, group := range file.Comments {
						for _, comment := range group.List {
							if pattern, ok := strings.CutPrefix(comment.Text, "//go:embed "); ok {
								info, err := os.Stat(filepath.Join(from, pattern))
								if strings.ContainsAny(pattern, "/\\*?[] \t") || strings.HasSuffix(pattern, "_test.go") || err != nil || !info.Mode().IsRegular() {
									return fmt.Errorf("%s/%s: embed pattern %q needs update support", path, name, pattern)
								}
							}
						}
					}
					// Replace only import literals, preserving upstream code and comments.
					for _, imp := range slices.Backward(file.Imports) {
						importPath, err := strconv.Unquote(imp.Path.Value)
						if err != nil {
							return err
						}
						for _, dep := range sources {
							if rest, ok := strings.CutPrefix(importPath, dep.prefix); ok {
								queue = append(queue, importPath)
								replacement := strconv.Quote(module + "/" + dep.local + rest)
								start, end := fset.Position(imp.Path.Pos()).Offset, fset.Position(imp.Path.End()).Offset
								data = bytes.Join([][]byte{data[:start], []byte(replacement), data[end:]}, nil)
							}
						}
					}
					data, err = format.Source(data)
					if err != nil {
						return err
					}
				}
				if err := write(filepath.Join(to, name), data); err != nil {
					return err
				}
			}
		}
	}
	for i, upstream := range []download{gopls, tools} {
		dest := filepath.Join(stage, strings.TrimSuffix(sources[i].local, "/"))
		license, err := os.ReadFile(filepath.Join(upstream.Dir, "LICENSE"))
		if err != nil {
			return err
		}
		if err := write(filepath.Join(dest, "LICENSE"), license); err != nil {
			return err
		}
		provenance := fmt.Sprintf("Copied from %s@%s\nModule checksum: %s\n\nGenerated by go run ./hack/update. Do not edit these sources manually.\nOnly imports and gofmt formatting are changed. Upstream tests are omitted.\n", upstream.Path, upstream.Version, upstream.Sum)
		if err := write(filepath.Join(dest, "UPSTREAM"), []byte(provenance)); err != nil {
			return err
		}
	}
	// These are the two generated directories owned by this command.
	for _, dir := range []string{"internal/gopls", "internal/xtools"} {
		if err := os.RemoveAll(dir); err != nil {
			return err
		}
		// The system temporary directory may be on another filesystem.
		if err := os.CopyFS(dir, os.DirFS(filepath.Join(stage, dir))); err != nil {
			return err
		}
	}
	if err := goRun("mod", "edit", "-go="+mod.Go, "-require=golang.org/x/tools@"+tools.Version); err != nil {
		return err
	}
	return goRun("mod", "tidy")
}

func fetch(module string) (download, error) {
	var result download
	err := goJSON(&result, "mod", "download", "-json", module)
	return result, err
}

func goJSON(value any, args ...string) error {
	cmd := exec.Command("go", args...)
	cmd.Stderr = os.Stderr
	data, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("go %s: %w\n%s", strings.Join(args, " "), err, data)
	}
	return json.Unmarshal(data, value)
}

func goRun(args ...string) error {
	cmd := exec.Command("go", args...)
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	return cmd.Run()
}

func write(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}
