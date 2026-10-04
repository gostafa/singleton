# zizmor

A [TaskOtter](https://github.com/task-otter/store) module for [zizmor](https://github.com/woodruffw/zizmor) — a security auditor for GitHub Actions workflow files.

## What is this Taskfile?

This module audits GitHub Actions workflows with [zizmor](https://github.com/woodruffw/zizmor). zizmor detects expression injection, excessive permissions, use of mutable actions, and other dangerous patterns. The `ci` task auto-installs zizmor via `nix:install:profile`.

## Usage

### Standalone

```sh
task -t taskfiles/zizmor/Taskfile.yml ci
task -t taskfiles/zizmor/Taskfile.yml ci ZIZMOR_TARGETS=.github/workflows/main.yml
task -t taskfiles/zizmor/Taskfile.yml ci ZIZMOR_EXTRA_ARGS="--min-severity high"
task -t taskfiles/zizmor/Taskfile.yml ci ZIZMOR_EXTRA_ARGS="--gh-token $GITHUB_TOKEN"
```

Install only, without auditing:

```sh
task -t taskfiles/zizmor/Taskfile.yml install
```

### Included in your Taskfile

```yaml
includes:
  zizmor:
    taskfile: taskfiles/zizmor/Taskfile.yml
```

Then run:

```sh
task zizmor:ci
task zizmor:install
task zizmor:version
```

## Public Tasks

| Task      | Description                                        |
| --------- | -------------------------------------------------- |
| `ci`      | Audit GitHub Actions workflows for security issues |
| `install` | Install zizmor via Nix (Unix) or WinGet (Windows)  |
| `version` | Show the active zizmor version                     |

## Variables

| Variable                    | Default          | Description                                                                           |
| --------------------------- | ---------------- | ------------------------------------------------------------------------------------- |
| `ZIZMOR_NIX_INSTALLABLE`    | `nixpkgs#zizmor` | Flake installable passed to `nix:install:profile`                                     |
| `ZIZMOR_WINGET_INSTALLABLE` | `zizmor.zizmor`  | WinGet package ID for `winget:install:package`                                        |
| `ZIZMOR_EXTRA_ARGS`         | `"--offline"`    | Additional flags passed to `zizmor` (e.g. `--format`, `--min-severity`, `--gh-token`) |
| `ZIZMOR_TARGETS`            | `".github"`      | Path to audit; scans workflows and composite actions under `.github`                  |

Pin a revision by overriding the installable, for example
`ZIZMOR_NIX_INSTALLABLE=github:NixOS/nixpkgs/<rev>#zizmor`.

## Notes

- Install uses Nix on Linux and macOS (`ZIZMOR_NIX_INSTALLABLE`) and WinGet on Windows (`ZIZMOR_WINGET_INSTALLABLE`, default `zizmor.zizmor`).

- The `ci` task auto-installs zizmor if it is not already present in `PATH`.
