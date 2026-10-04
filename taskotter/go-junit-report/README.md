# go-junit-report Taskfile

## What is this Taskfile?

A Taskfile for converting an existing `go test` log file to JUnit XML via
`go-junit-report`. The converter is installed through `nix:install:profile`.
Go is provided by the included [`go`](../go/README.md) Taskfile (`go:install`),
not by this module's installable.

For plain `go test` without report artifacts, use the [`go`](../go/README.md)
Taskfile.

## Usage

### Standalone

```sh
task -t taskfiles/go-junit-report/Taskfile.yml verify
task -t taskfiles/go-junit-report/Taskfile.yml report GO_JUNIT_REPORT_IN=gotest.log GO_JUNIT_REPORT_OUT=junit.xml
```

### Included

```yaml
includes:
  go-junit-report: ./taskfiles/go-junit-report/Taskfile.yml
```

Then run:

```sh
task go-junit-report:verify
task go-junit-report:report GO_JUNIT_REPORT_IN=gotest.log GO_JUNIT_REPORT_OUT=junit.xml
```

Install only the converter binary (Go comes from the `go` module when needed):

```sh
task go-junit-report:install
```

## Reporting

Convert an existing go test log file to JUnit XML:

```sh
task go-junit-report:report GO_JUNIT_REPORT_IN=gotest.log GO_JUNIT_REPORT_OUT=junit.xml
```

`report` runs `go-junit-report -in … -out … -set-exit-code`. It does not run
`go test` or write coverage profiles. Produce the input log separately (for
example with `go test -v ./... > gotest.log`), then pass explicit in/out paths.

Pin a revision by overriding the installable, for example
`GO_JUNIT_REPORT_NIX_INSTALLABLE=github:NixOS/nixpkgs/<rev>#go-junit-report`.

Override the Go version via `GO_NIX_INSTALLABLE` on the go module before
dependents run `go:install`.

## Public Tasks

| Task      | Description                                                    |
| --------- | -------------------------------------------------------------- |
| `which`   | Show the path to the go-junit-report binary                    |
| `verify`  | Print go-junit-report version                                  |
| `report`  | Convert a go test log file to JUnit XML                        |
| `install` | Install go-junit-report via Nix (Unix) or go install (Windows) |
| `version` | Show the active go-junit-report version                        |

## Variables

| Variable                          | Default                                         | Description                                       |
| --------------------------------- | ----------------------------------------------- | ------------------------------------------------- |
| `GO_JUNIT_REPORT_NIX_INSTALLABLE` | `nixpkgs#go-junit-report`                       | Flake installable passed to `nix:install:profile` |
| `GO_JUNIT_REPORT_GO_PKG`          | `github.com/jstemmer/go-junit-report/v2@latest` | Go module passed to `go:install:pkg` on Windows   |
| `GO_JUNIT_REPORT_IN`              | empty (required)                                | Input path of the go test log file                |
| `GO_JUNIT_REPORT_OUT`             | empty (required)                                | Output path for the JUnit XML report              |

## Notes

- Unix install uses Nix (`GO_JUNIT_REPORT_NIX_INSTALLABLE`). On Windows, install uses `go:install:pkg` with `GO_JUNIT_REPORT_GO_PKG`.

- `report`, `which`, `verify`, and `version` auto-install `go-junit-report`.
  Go is installed through the included module only when the Windows installer needs it.
- `report` validates required paths, a readable input file, and a writable output
  location before installation. Input and output must differ.
