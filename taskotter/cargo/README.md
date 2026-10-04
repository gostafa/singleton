# cargo

A [TaskOtter](https://github.com/task-otter/store) module for the
[Rust](https://www.rust-lang.org/) toolchain and its [Cargo](https://doc.rust-lang.org/cargo/)
build tool.

## What is this Taskfile?

This module runs the common Cargo workflow (build, test, check, format, and
lint). The Rust toolchain is installed through `nix:install:profile`.

## Usage

### Standalone

```sh
task nix:install:profile NIX_INSTALLABLE=nixpkgs#cargo
task -t taskfiles/cargo/Taskfile.yml build
task -t taskfiles/cargo/Taskfile.yml test
task -t taskfiles/cargo/Taskfile.yml lint
```

### Included in your Taskfile

```yaml
includes:
  cargo:
    taskfile: taskfiles/cargo/Taskfile.yml
```

Then run:

```sh
task cargo:build
task cargo:fmt
task cargo:lint
```

Pin a revision by overriding the installable, for example
`CARGO_NIX_INSTALLABLE=github:NixOS/nixpkgs/<rev>#cargo`.

## Building and testing

Cargo tasks run in the directory where you invoke `task`, so run them from a
crate root (where `Cargo.toml` lives). Pass extra Cargo flags with `CARGO_EXTRA_ARGS`:

```sh
task cargo:build CARGO_EXTRA_ARGS=--release
task cargo:test CARGO_EXTRA_ARGS="-- --nocapture"
task cargo:check
```

## Formatting and linting

```sh
task cargo:fmt              # cargo fmt
task cargo:fmt:check        # cargo fmt --check (reports without writing)
task cargo:lint             # cargo clippy
task cargo:lint:fix         # cargo clippy --fix
task cargo:lint CARGO_EXTRA_ARGS="-- -D warnings"
```

## Toolchains

Leave `RUST_TOOLCHAIN` empty to use the cargo from the Nix profile. Set it to a
channel or version to invoke Cargo as `cargo +<toolchain>` when rustup is also
present. Pinning the Nix package uses `CARGO_NIX_INSTALLABLE`.

```sh
task cargo:build RUST_TOOLCHAIN=1.79.0 CARGO_EXTRA_ARGS=--release
```

## Public Tasks

| Task            | Description                                          |
| --------------- | ---------------------------------------------------- |
| `build`         | Build the crate with `cargo build`                   |
| `check`         | Type-check the crate with `cargo check`              |
| `test`          | Run the crate test suite with `cargo test`           |
| `fmt`           | Format Rust code with `cargo fmt`                    |
| `fmt:check`     | Check Rust formatting with `cargo fmt --check`       |
| `lint`          | Lint Rust code with `cargo clippy`                   |
| `lint:fix`      | Auto-fix Rust lint issues with `cargo clippy --fix`  |
| `ci`            | Run `fmt:check` then `lint`                          |
| `ci:fix`        | Run `fmt` then `lint:fix` for CI                     |
| `which`         | Show the path to the cargo binary                    |
| `verify`        | Print cargo and rustc versions                       |
| `install`       | Install Cargo via Nix (Unix) or WinGet (Windows)     |
| `install:crate` | Install a crate with `cargo install` (`CARGO_CRATE`) |
| `version`       | Show the active Cargo version                        |

## Variables

| Variable                   | Default                                                | Description                                                                                                  |
| -------------------------- | ------------------------------------------------------ | ------------------------------------------------------------------------------------------------------------ |
| `CARGO_NIX_INSTALLABLE`    | `nixpkgs#cargo`                                        | Flake installable passed to `nix:install:profile`                                                            |
| `CARGO_WINGET_INSTALLABLE` | `Rustlang.Rustup`                                      | WinGet package ID for `winget:install:package`                                                               |
| `CARGO_LOAD`               | reloads User Path; prepends `%USERPROFILE%\.cargo\bin` | PowerShell snippet so `cargo` and cargo-installed binaries from earlier in the same Task process are on PATH |
| `RUST_TOOLCHAIN`           | empty                                                  | Optional toolchain channel or version, such as `nightly` or `1.79.0`                                         |
| `CARGO_EXTRA_ARGS`         | empty                                                  | Extra flags appended to Cargo subcommands                                                                    |
| `CARGO_CRATE`              | empty                                                  | Crate name for `install:crate`; required when running that task                                              |

## Notes

- Install uses Nix on Linux and macOS (`CARGO_NIX_INSTALLABLE`) and WinGet on Windows (`CARGO_WINGET_INSTALLABLE`, default `Rustlang.Rustup`).

- On Windows, `CARGO_LOAD` reloads User Path and prepends `%USERPROFILE%\.cargo\bin` so cargo-installed binaries from earlier in the same Task process are on PATH.

- `fmt` and `lint` need `rustfmt` and `clippy` on PATH. Override `CARGO_NIX_INSTALLABLE` to add them, for example `nixpkgs#cargo nixpkgs#clippy nixpkgs#rustfmt`.
