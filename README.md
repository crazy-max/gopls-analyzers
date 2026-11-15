<p align="center">
  <a href="https://pkg.go.dev/github.com/crazy-max/gopls-analyzers/cmd/gopls-analyzers"><img src="https://img.shields.io/badge/go.dev-docs-007d9c?logo=go&logoColor=white&style=flat-square" alt="PkgGoDev"></a>
  <a href="https://github.com/crazy-max/gopls-analyzers/releases/latest"><img src="https://img.shields.io/github/release/crazy-max/gopls-analyzers.svg?style=flat-square" alt="GitHub release"></a>
  <a href="https://github.com/crazy-max/gopls-analyzers/releases/latest"><img src="https://img.shields.io/github/downloads/crazy-max/gopls-analyzers/total.svg?style=flat-square" alt="Total downloads"></a>
  <a href="https://github.com/crazy-max/gopls-analyzers/actions?workflow=build"><img src="https://img.shields.io/github/actions/workflow/status/crazy-max/gopls-analyzers/build.yml?label=build&logo=github&style=flat-square" alt="Build Status"></a>
  <a href="https://hub.docker.com/r/crazymax/gopls-analyzers/"><img src="https://img.shields.io/docker/stars/crazymax/gopls-analyzers.svg?style=flat-square&logo=docker" alt="Docker Stars"></a>
  <a href="https://hub.docker.com/r/crazymax/gopls-analyzers/"><img src="https://img.shields.io/docker/pulls/crazymax/gopls-analyzers.svg?style=flat-square&logo=docker" alt="Docker Pulls"></a>
  <br /><a href="https://codecov.io/gh/crazy-max/gopls-analyzers"><img src="https://img.shields.io/codecov/c/github/crazy-max/gopls-analyzers?logo=codecov&style=flat-square" alt="Codecov"></a>
  <a href="https://github.com/sponsors/crazy-max"><img src="https://img.shields.io/badge/sponsor-crazy--max-181717.svg?logo=github&style=flat-square" alt="Become a sponsor"></a>
  <a href="https://www.paypal.me/crazyws"><img src="https://img.shields.io/badge/donate-paypal-00457c.svg?logo=paypal&style=flat-square" alt="Donate Paypal"></a>
</p>

## About

Run gopls analyzers with a single go vet-compatible tool.

Selected analyzers run together in one executable, with flags to choose
which checks to run. Their sources and internal dependencies are copied from
a pinned gopls release. See [hack/gopls.version](hack/gopls.version) for the
current version.

## Installation

```console
$ go install github.com/crazy-max/gopls-analyzers/cmd/gopls-analyzers@latest
```

## Usage

Run from a Go project, using the path to the installed binary:

```console
$ go vet -vettool=/absolute/path/to/gopls-analyzers ./...
```

The analysis environment needs Go and the target project's dependencies, just
as it does for `go vet`.

### Select analyzers

All bundled analyzers run by default. Passing positive analyzer flags selects
only those analyzers:

```console
$ go vet -vettool=/absolute/path/to/gopls-analyzers -unusedparams -infertypeargs ./...
```

Passing only negative flags disables those analyzers and keeps the rest:

```console
$ go vet -vettool=/absolute/path/to/gopls-analyzers -unusedparams=false ./...
```

Inspect the available analyzers and their documentation:

```console
$ gopls-analyzers help
$ gopls-analyzers help unusedparams
```

### Exclude files

Use `-exclude-generated` to suppress diagnostics and fixes in files with a
standard `// Code generated ... DO NOT EDIT.` marker:

```console
$ go vet -vettool=/absolute/path/to/gopls-analyzers -exclude-generated ./...
```

Use `-exclude` with a regular expression to exclude other files. It matches
absolute file paths with `/` separators on every platform:

```console
$ go vet -vettool=/absolute/path/to/gopls-analyzers -exclude='/(pb|third_party)/' ./...
```

Both flags are optional and can be combined. They also work when running the
executable directly, including with `-fix`. Excluded files remain available
for parsing and type checking; these flags suppress analyzer diagnostics,
not compiler errors. Fixes that would edit an excluded file are suppressed
in their entirety, even when reported in another file.

### Apply fixes

The executable also accepts package patterns directly, including fixes:

```console
$ gopls-analyzers -infertypeargs ./...
$ gopls-analyzers -infertypeargs -fix ./...
```

Review fixes before committing them. Diagnostics cause a nonzero exit status;
successful checks are silent.

### Bundled analyzers

| Analyzer                                                                    | Flag                    | Description                                                                |
| --------------------------------------------------------------------------- | ----------------------- | -------------------------------------------------------------------------- |
| [embeddirective](https://go.dev/gopls/analyzers#embed)                      | `-embed`                | Checks `//go:embed` directives and their required import.                  |
| [fillreturns](https://go.dev/gopls/analyzers#fillreturns)                   | `-fillreturns`          | Suggests zero values for missing return values.                            |
| [infertypeargs](https://go.dev/gopls/analyzers#infertypeargs)               | `-infertypeargs`        | Removes explicit type arguments that Go can infer.                         |
| [maprange](https://go.dev/gopls/analyzers#maprange)                         | `-maprange`             | Replaces redundant `maps.Keys` or `maps.Values` calls in range loops.      |
| [nonewvars](https://go.dev/gopls/analyzers#nonewvars)                       | `-nonewvars`            | Replaces `:=` with `=` when no new variables are declared.                 |
| [noresultvalues](https://go.dev/gopls/analyzers#noresultvalues)             | `-noresultvalues`       | Removes return values from functions that return no results.               |
| [simplifycompositelit](https://go.dev/gopls/analyzers#simplifycompositelit) | `-simplifycompositelit` | Removes redundant types from nested composite literals.                    |
| [simplifyslice](https://go.dev/gopls/analyzers#simplifyslice)               | `-simplifyslice`        | Simplifies slice expressions such as `s[a:len(s)]` to `s[a:]`.             |
| [unusedparams](https://go.dev/gopls/analyzers#unusedparams)                 | `-unusedparams`         | Reports unused parameters in eligible unexported functions and methods.    |
| [yield](https://go.dev/gopls/analyzers#yield)                               | `-yield`                | Reports iterator paths that may call `yield` again after it returns false. |

`fillreturns`, `nonewvars`, and `noresultvalues` provide fixes for type errors.
The `go vet` driver stops on those errors before running the analyzers. Run
the executable directly on package patterns to use these checks.

Upstream's `unusedparams` analyzer assumes gopls's package-loading behavior.
Other drivers can report false positives when a function is used as a value
only in test files. Disable it with `-unusedparams=false` if that affects your
project.

### Modernize

The executable also bundles the
[modernize suite](https://pkg.go.dev/golang.org/x/tools/go/analysis/passes/modernize)
from x/tools. Its checks run by default alongside the gopls analyzers.
Select or disable individual checks using their names, such as `-any`,
`-rangeint`, or `-any=false`. There is no combined `-modernize` flag.

For example, check for `interface{}` declarations that can use `any`:

```console
$ go vet -vettool=/absolute/path/to/gopls-analyzers -any ./...
```

Apply that check's fixes directly:

```console
$ gopls-analyzers -any -fix ./...
```

Run `gopls-analyzers help` to list all bundled checks.

### Container image

Copy the binary from `crazymax/gopls-analyzers:edge` into an environment
that has Go:

```dockerfile
FROM crazymax/gopls-analyzers AS analyzers
FROM golang
COPY --from=analyzers /usr/local/bin/gopls-analyzers /usr/local/bin/
WORKDIR /src
RUN --mount=type=bind,target=. \
    go vet -vettool=/usr/local/bin/gopls-analyzers ./...
```

The distribution image itself has no Go toolchain or shell. Its executable
must match the analysis host, even when analyzing code for another platform.

## License

MIT. See [`LICENSE`](LICENSE) for more details.

Copied Go sources retain their upstream copyright notices and BSD licenses
in [internal/gopls/LICENSE](internal/gopls/LICENSE) and
[internal/xtools/LICENSE](internal/xtools/LICENSE).
