# Support

## Reporting an issue

Read the [README](../README.md), including the documented analyzer limitations,
and search existing issues before opening a new one:
https://github.com/crazy-max/gopls-analyzers/issues?q=is%3Aissue

If an issue already describes your problem, add relevant details or a reaction
instead of a "+1" comment.

For usage questions and discussions, use
https://github.com/crazy-max/gopls-analyzers/discussions.

## Writing good bug reports and feature requests

File one issue per bug or feature request. For bug reports, include:

* The gopls-analyzers release tag or commit and installation method.
* The output of `go version` and `go env GOOS GOARCH GOTOOLCHAIN`.
* The exact command and analyzer flags you used.
* A minimal Go example and relevant `go.mod` directives to reproduce the issue.
* The expected behavior and actual diagnostic output.
* The image tag or digest if you use the binary from a container image.

For feature requests, describe the problem you want to solve. For analyzer
requests, include example Go code and the expected diagnostic or fix.

The gopls analyzers are copied from the release pinned in
[hack/gopls.version](../hack/gopls.version). The modernize checks come from
`golang.org/x/tools`. If you have reproduced the problem in upstream gopls or
modernize, mention that and include any existing upstream issue URL.

Create a new issue at
https://github.com/crazy-max/gopls-analyzers/issues/new/choose.

## Closure policy

* Issues missing the requested information may be closed with a request to
  follow these guidelines.
* Issues awaiting a response from the original poster for a week may be closed
  at the maintainer's discretion.
