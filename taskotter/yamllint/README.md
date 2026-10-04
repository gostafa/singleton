# yamllint Taskfile Public Tasks

## What is this Taskfile?

A cross-platform Taskfile for linting YAML files and generating a project
configuration. The `ci` task auto-installs yamllint via `nix:install:profile`.

## Usage

### Standalone

```sh
task -t taskfiles/yamllint/Taskfile.yml config:init
task -t taskfiles/yamllint/Taskfile.yml ci
```

Install only:

```sh
task -t taskfiles/yamllint/Taskfile.yml install
```

### Included

```yaml
includes:
  yamllint: ./taskfiles/yamllint/Taskfile.yml
```

Then run:

```sh
task yamllint:ci
```

## Public Tasks

| Task          | Description                                          |
| ------------- | ---------------------------------------------------- |
| `ci`          | Strict lint for CI (fails on warnings)               |
| `config:init` | Create a default `.yamllint` configuration file      |
| `install`     | Install yamllint via Nix (Unix) or uv tool (Windows) |
| `version`     | Show the active yamllint version                     |

## Variables

| Variable                   | Default            | Description                                         |
| -------------------------- | ------------------ | --------------------------------------------------- |
| `YAMLLINT_NIX_INSTALLABLE` | `nixpkgs#yamllint` | Flake installable passed to `nix:install:profile`   |
| `YAMLLINT_UV_TOOL`         | `yamllint`         | uv tool name passed to `uv:tool:install` on Windows |
| `YAMLLINT_TARGETS`         | `.`                | Files or directories to lint                        |
| `YAMLLINT_CONFIG`          | _(empty)_          | Path to a yamllint config file passed via `-c`      |
| `YAMLLINT_EXTRA_ARGS`      | _(empty)_          | Extra flags forwarded to `yamllint`                 |

Pin a revision by overriding the installable, for example
`YAMLLINT_NIX_INSTALLABLE=github:NixOS/nixpkgs/<rev>#yamllint`.

## Notes

- Unix install uses Nix (`YAMLLINT_NIX_INSTALLABLE`). Windows installs via `uv:tool:install` (`YAMLLINT_UV_TOOL`).

**`config:init`** writes a `.yamllint` file in the current directory and is
skipped if the file already exists. To regenerate, delete `.yamllint` first.

For YAML auto-formatting, use the separate [`yamlfix`](../yamlfix/README.md) module.
