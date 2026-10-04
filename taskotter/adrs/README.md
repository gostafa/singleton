# adrs Taskfile

## What is this Taskfile?

This Taskfile wraps [adrs](https://github.com/joshrotenberg/adrs), a
command-line tool for managing Architecture Decision Records. Remaining tasks
auto-install adrs via `nix:install:profile`.

## Usage

### Standalone

```bash
task --taskfile taskfiles/adrs/Taskfile.yml list
```

Install only:

```sh
task -t taskfiles/adrs/Taskfile.yml install
```

### Included

```yaml
includes:
  adrs:
    taskfile: taskfiles/adrs/Taskfile.yml
```

```bash
task adrs:init
task adrs:new -- "Use PostgreSQL for persistence"
task adrs:list
task adrs:generate -- toc
task adrs:exec -- search postgres
```

Pass arguments and flags with `ADRS_EXTRA_ARGS=...` or after `--`.

## Public Tasks

| Task       | Description                                            |
| ---------- | ------------------------------------------------------ |
| `init`     | Initialize an ADR repository                           |
| `new`      | Create a new ADR                                       |
| `list`     | List all ADRs                                          |
| `generate` | Generate ADR docs (`toc`, `graph`, or `book`)          |
| `exec`     | Run any adrs subcommand                                |
| `install`  | Install adrs via Nix (Unix) or cargo install (Windows) |
| `version`  | Show the active adrs version                           |

## Variables

| Variable               | Default        | Description                                         |
| ---------------------- | -------------- | --------------------------------------------------- |
| `ADRS_NIX_INSTALLABLE` | `nixpkgs#adrs` | Flake installable passed to `nix:install:profile`   |
| `ADRS_CARGO_CRATE`     | `adrs`         | Crate name for Windows `cargo:install:crate`        |
| `ADRS_EXTRA_ARGS`      | `""`           | Arguments and flags appended to the adrs subcommand |

Pin a revision by overriding the installable, for example
`ADRS_NIX_INSTALLABLE=github:NixOS/nixpkgs/<rev>#adrs`.

## Notes

- Unix install uses Nix (`ADRS_NIX_INSTALLABLE`). Windows installs via
  `cargo:install:crate` (`ADRS_CARGO_CRATE`).
