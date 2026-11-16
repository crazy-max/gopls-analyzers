package main

import (
	"flag"
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"sync"
	"time"

	"github.com/crazy-max/gopls-analyzers/internal/gopls/analysis/embeddirective"
	"github.com/crazy-max/gopls-analyzers/internal/gopls/analysis/fillreturns"
	"github.com/crazy-max/gopls-analyzers/internal/gopls/analysis/infertypeargs"
	"github.com/crazy-max/gopls-analyzers/internal/gopls/analysis/maprange"
	"github.com/crazy-max/gopls-analyzers/internal/gopls/analysis/nonewvars"
	"github.com/crazy-max/gopls-analyzers/internal/gopls/analysis/noresultvalues"
	"github.com/crazy-max/gopls-analyzers/internal/gopls/analysis/simplifycompositelit"
	"github.com/crazy-max/gopls-analyzers/internal/gopls/analysis/simplifyslice"
	"github.com/crazy-max/gopls-analyzers/internal/gopls/analysis/unusedparams"
	"github.com/crazy-max/gopls-analyzers/internal/gopls/analysis/yield"
	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/multichecker"
	"golang.org/x/tools/go/analysis/passes/modernize"
)

var version = "dev"

func main() {
	flag.BoolFunc("version", "print version and exit", func(string) error {
		fmt.Println(version)
		os.Exit(0)
		return nil
	})

	verbose := flag.Bool("verbose", false, "log package analysis progress to stderr")
	verboseAnalyzers := flag.Bool("verbose-analyzers", false, "log package progress and individual analyzer execution times to stderr")

	var exclude exclusions
	flag.BoolVar(&exclude.generated, "exclude-generated", false, "exclude diagnostics and fixes in generated files")
	flag.Func("exclude", "exclude diagnostics and fixes in files whose absolute slash-separated path matches this regular expression", func(value string) error {
		pattern, err := regexp.Compile(value)
		if err != nil {
			return err
		}
		exclude.pattern = pattern
		return nil
	})

	analyzers := []*analysis.Analyzer{
		embeddirective.Analyzer,
		fillreturns.Analyzer,
		infertypeargs.Analyzer,
		maprange.Analyzer,
		nonewvars.Analyzer,
		noresultvalues.Analyzer,
		simplifycompositelit.Analyzer,
		simplifyslice.Analyzer,
		unusedparams.Analyzer,
		yield.Analyzer,
	}
	analyzers = append(analyzers, modernize.Suite...)

	flag.BoolFunc("list-analyzers", "list all bundled analyzers and exit", func(value string) error {
		list, err := strconv.ParseBool(value)
		if err != nil {
			return err
		}
		if !list {
			return nil
		}
		names := make([]string, 0, len(analyzers))
		for _, analyzer := range analyzers {
			names = append(names, analyzer.Name)
		}
		slices.Sort(names)
		fmt.Printf("gopls-analyzers: bundled analyzers (%d):\n", len(names))
		for _, name := range names {
			fmt.Printf("  - %s\n", name)
		}
		os.Exit(0)
		return nil
	})

	var logMu sync.Mutex
	loggedPackages := make(map[*types.Package]bool)
	for _, analyzer := range analyzers {
		run := analyzer.Run
		analyzer.Run = func(pass *analysis.Pass) (any, error) {
			if !*verbose && !*verboseAnalyzers {
				return run(exclude.filter(pass))
			}
			// Key by package identity so direct mode also logs test variants.
			logMu.Lock()
			if !loggedPackages[pass.Pkg] {
				fmt.Fprintf(os.Stderr, "gopls-analyzers: %s: analyzing (files: %d)\n", pass.Pkg.Path(), len(pass.Files))
				loggedPackages[pass.Pkg] = true
			}
			logMu.Unlock()
			if !*verboseAnalyzers {
				return run(exclude.filter(pass))
			}
			fmt.Fprintf(os.Stderr, "gopls-analyzers: %s: %s: starting (files: %d)\n", pass.Pkg.Path(), pass.Analyzer.Name, len(pass.Files))
			start := time.Now()
			result, err := run(exclude.filter(pass))
			if err != nil {
				fmt.Fprintf(os.Stderr, "gopls-analyzers: %s: %s: failed after %s: %v\n", pass.Pkg.Path(), pass.Analyzer.Name, time.Since(start), err)
			} else {
				fmt.Fprintf(os.Stderr, "gopls-analyzers: %s: %s: completed in %s\n", pass.Pkg.Path(), pass.Analyzer.Name, time.Since(start))
			}
			return result, err
		}
	}

	multichecker.Main(analyzers...)
}

type exclusions struct {
	generated bool
	pattern   *regexp.Regexp
}

func (e exclusions) filter(pass *analysis.Pass) *analysis.Pass {
	if !e.generated && e.pattern == nil {
		return pass
	}
	excluded := make(map[*token.File]bool)
	for _, file := range pass.Files {
		tokenFile := pass.Fset.File(file.Pos())
		excluded[tokenFile] = (e.generated && ast.IsGenerated(file)) ||
			(e.pattern != nil && e.pattern.MatchString(filepath.ToSlash(tokenFile.Name())))
	}
	isExcluded := func(pos token.Pos) bool {
		return excluded[pass.Fset.File(pos)]
	}

	filtered := *pass
	filtered.Report = func(diagnostic analysis.Diagnostic) {
		if isExcluded(diagnostic.Pos) {
			return
		}
		// A fix reported in a handwritten file may also edit an excluded file.
		diagnostic.SuggestedFixes = slices.DeleteFunc(slices.Clone(diagnostic.SuggestedFixes), func(fix analysis.SuggestedFix) bool {
			return slices.ContainsFunc(fix.TextEdits, func(edit analysis.TextEdit) bool {
				return isExcluded(edit.Pos)
			})
		})
		pass.Report(diagnostic)
	}
	return &filtered
}
