// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build ignore

// The simplifyslice command applies the golang.org/x/tools/gopls/internal/analysis/simplifyslice
// analysis to the specified packages of Go source code.
package main

import (
	"github.com/crazy-max/gopls-analyzers/internal/gopls/analysis/simplifyslice"
	"golang.org/x/tools/go/analysis/singlechecker"
)

func main() { singlechecker.Main(simplifyslice.Analyzer) }
