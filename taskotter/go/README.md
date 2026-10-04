# Go Taskfile

## What is this Taskfile?

A Taskfile for running Go unit tests, benchmarks, and fuzz targets. The Go
toolchain is installed through `nix:install:profile`.

Linting and formatting live in the [`golangci-lint`](../golangci-lint/README.md)
Taskfile, and JUnit XML conversion lives in the
[`go-junit-report`](../go-junit-report/README.md) Taskfile.

## Usage

### Standalone

```sh
task nix:install:profile NIX_INSTALLABLE=nixpkgs#go
task -t taskfiles/go/Taskfile.yml verify
task -t taskfiles/go/Taskfile.yml test
```

### Included

```yaml
includes:
  go: ./taskfiles/go/Taskfile.yml
```

Then run:

```sh
task go:verify
task go:test
```

Install only, without running tests:

```sh
task nix:install:profile NIX_INSTALLABLE=nixpkgs#go
```

## Testing

Run unit tests, benchmarks, and fuzz targets against the current project:

```sh
task go:test
task go:bench
task go:fuzz -- -fuzz FuzzName ./internal/parser
```

Each task defaults to the `./...` package pattern (except `fuzz`, which fuzzes
one target in a single package). Pass extra `go test` flags or a narrower target
after `--`:

```sh
task go:test -- -race -run TestName ./internal/...
task go:bench -- -bench BenchmarkName ./internal/parser
```

`test` runs plain `go test -v`. For converting a go test log to JUnit XML, use
`go-junit-report:report`.
`fuzz` runs a single target for `GO_FUZZTIME` (default `30s`); Go fuzzes one
target in one package per run, so supply the `-fuzz` pattern and package after
`--`:

```sh
task go:fuzz GO_FUZZTIME=60s -- -fuzz FuzzName ./internal/parser
```

Pin a revision by overriding the installable, for example
`GO_NIX_INSTALLABLE=github:NixOS/nixpkgs/<rev>#go`.

## Public Tasks

| Task          | Description                                       |
| ------------- | ------------------------------------------------- |
| `which`       | Show the path to the Go binary                    |
| `verify`      | Print Go version, GOROOT, and GOPATH              |
| `test`        | Run Go unit tests                                 |
| `bench`       | Run Go benchmarks                                 |
| `fuzz`        | Run a Go fuzz target                              |
| `install`     | Install Go via Nix (Unix) or WinGet (Windows)     |
| `install:pkg` | Install a Go package with `go install` (`GO_PKG`) |
| `version`     | Show the active Go version                        |

## Variables

| Variable                | Default                                | Description                                                                                  |
| ----------------------- | -------------------------------------- | -------------------------------------------------------------------------------------------- |
| `GO_NIX_INSTALLABLE`    | `nixpkgs#go`                           | Flake installable passed to `nix:install:profile`                                            |
| `GO_WINGET_INSTALLABLE` | `GoLang.Go`                            | WinGet package ID for `winget:install:package`                                               |
| `GO_FUZZTIME`           | empty (`30s`)                          | Duration a single `fuzz` target runs before stopping                                         |
| `GO_PKG`                | empty                                  | Module path for `install:pkg`; required when running that task                               |
| `GO_LOAD`               | reloads User Path; prepends GOPATH\bin | PowerShell snippet so `go`-installed tools from earlier in the same Task process are on PATH |

## Notes

- Install uses Nix on Linux and macOS (`GO_NIX_INSTALLABLE`) and WinGet on Windows (`GO_WINGET_INSTALLABLE`, default `GoLang.Go`).

- `test`, `bench`, `fuzz`, `which`, `verify`, and `install:pkg` auto-install Go.

- On Windows, `install:pkg` prepends `$(go env GOPATH)\bin` to the User Path when missing.
