// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build ignore

// The embeddirective command applies the golang.org/x/tools/gopls/internal/analysis/embeddirective
// analysis to the specified packages of Go source code.
package main

import (
	"github.com/crazy-max/gopls-analyzers/internal/gopls/analysis/embeddirective"
	"golang.org/x/tools/go/analysis/singlechecker"
)

func main() { singlechecker.Main(embeddirective.Analyzer) }
