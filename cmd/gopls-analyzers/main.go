package main

import (
	"flag"
	"go/ast"
	"go/token"
	"path/filepath"
	"regexp"
	"slices"

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

func main() {
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

	for _, analyzer := range analyzers {
		run := analyzer.Run
		analyzer.Run = func(pass *analysis.Pass) (any, error) {
			return run(exclude.filter(pass))
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
