# yamlfix

A [TaskOtter](https://github.com/task-otter/store) module for [yamlfix](https://github.com/lyz-code/yamlfix), a YAML formatter and auto-fixer.

## What is this Taskfile?

This module formats YAML files in place. The `ci:fix` task auto-installs
yamlfix via `nix:install:profile`. It excludes `Taskfile.yml` / `Taskfile.yaml`
whose Go template syntax is incompatible with yamlfix.

## Usage

### Standalone

```sh
task -t taskfiles/yamlfix/Taskfile.yml ci:fix YAMLFIX_TARGETS=.
```

Install only:

```sh
task -t taskfiles/yamlfix/Taskfile.yml install
```

### Included in your Taskfile

```yaml
includes:
  yamlfix:
    taskfile: taskfiles/yamlfix/Taskfile.yml
```

Then run:

```sh
task yamlfix:ci:fix
```

## Public Tasks

| Task      | Description                                         |
| --------- | --------------------------------------------------- |
| `ci:fix`  | Auto-fix YAML files with yamlfix                    |
| `install` | Install yamlfix via Nix (Unix) or uv tool (Windows) |
| `version` | Show the active yamlfix version                     |

## Variables

| Variable                  | Default           | Description                                         |
| ------------------------- | ----------------- | --------------------------------------------------- |
| `YAMLFIX_NIX_INSTALLABLE` | `nixpkgs#yamlfix` | Flake installable passed to `nix:install:profile`   |
| `YAMLFIX_UV_TOOL`         | `yamlfix`         | uv tool name passed to `uv:tool:install` on Windows |
| `YAMLFIX_TARGETS`         | `.`               | Files or directories to format                      |
| `YAMLFIX_EXTRA_ARGS`      | _(empty)_         | Extra flags forwarded to `yamlfix`                  |

Pin a revision by overriding the installable, for example
`YAMLFIX_NIX_INSTALLABLE=github:NixOS/nixpkgs/<rev>#yamlfix`.

## Notes

- Unix install uses Nix (`YAMLFIX_NIX_INSTALLABLE`). Windows installs via `uv:tool:install` (`YAMLFIX_UV_TOOL`).

- `ci:fix` skips `Taskfile.yml` and `Taskfile.yaml` because Go template syntax breaks yamlfix.
