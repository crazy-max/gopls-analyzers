package main

import (
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
	multichecker.Main(append(analyzers, modernize.Suite...)...)
}
