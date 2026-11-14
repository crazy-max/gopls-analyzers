// Copyright 2024 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build ignore

// The yield command applies the yield analyzer to the specified
// packages of Go source code.
package main

import (
	"github.com/crazy-max/gopls-analyzers/internal/gopls/analysis/yield"
	"golang.org/x/tools/go/analysis/singlechecker"
)

func main() { singlechecker.Main(yield.Analyzer) }
