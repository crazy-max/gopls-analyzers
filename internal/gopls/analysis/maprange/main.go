// Copyright 2024 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build ignore

// The maprange command applies the golang.org/x/tools/gopls/internal/analysis/maprange
// analysis to the specified packages of Go source code.
package main

import (
	"github.com/crazy-max/gopls-analyzers/internal/gopls/analysis/maprange"
	"golang.org/x/tools/go/analysis/singlechecker"
)

func main() { singlechecker.Main(maprange.Analyzer) }
