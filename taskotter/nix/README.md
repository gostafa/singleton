# Nix Taskfile Public Tasks

## What is this Taskfile?

A Taskfile for installing, upgrading, and uninstalling
[Nix](https://nix.dev/manual/nix/2.34/installation/index.html) 2.34.

Installs follow the official binary installer. Linux with systemd and macOS
use the recommended **multi-user** (`--daemon`) install. Linux without
systemd uses **single-user** (`--no-daemon`). Windows WSL2 uses the Linux
path. Native Windows is not supported.

Upgrades and uninstalls follow the Nix 2.34 manual:
[upgrading](https://nix.dev/manual/nix/2.34/installation/upgrading.html) and
[uninstalling](https://nix.dev/manual/nix/2.34/installation/uninstall.html).

Most operations use `NIX_LOAD` so `nix` is reachable without a shell restart.

The rest of the store installs CLI tools through `nix:install:profile`.
Modules own a `{TOOL}_NIX_INSTALLABLE` and depend on that task instead of
shipping their own installers.

## Usage

### Standalone

```sh
task -t taskfiles/nix/Taskfile.yml install
task -t taskfiles/nix/Taskfile.yml install:shell NIX_INSTALLABLE=nixpkgs#hello NIX_COMMAND=hello
task -t taskfiles/nix/Taskfile.yml install:profile NIX_INSTALLABLE=nixpkgs#jq
task -t taskfiles/nix/Taskfile.yml uninstall
```

### Included

```yaml
includes:
  nix: ./taskfiles/nix/Taskfile.yml
```

Then run:

```sh
task nix:install
task nix:install:shell NIX_INSTALLABLE=hello NIX_COMMAND=hello
task nix:install:profile NIX_INSTALLABLE=nixpkgs#jq
```

## Auto-install behaviour

`version` and `upgrade` install Nix first if it is missing.

`install:shell` and `install:profile` auto-install Nix and enable the
`nix-command` and `flakes` experimental features (`NIX_NEEDED_FEATURES`) if
they are missing. They persist those flags in `nix.conf` and also pass
`--extra-experimental-features` on the `nix` command. They `require`
`NIX_INSTALLABLE` and fail the precondition if it is empty.

Installs are **idempotent**: the internal install task skips work when Nix is
already present (and matches `NIX_VERSION` when that is set).

## Public Tasks

| Task              | Description                                                           | Key variables                                      |
| ----------------- | --------------------------------------------------------------------- | -------------------------------------------------- |
| `install`         | Install Nix on the current OS if missing                              | `NIX_VERSION`                                      |
| `install:shell`   | Temporary `nix shell` with packages on PATH for one session           | `NIX_INSTALLABLE`, `NIX_COMMAND`, `NIX_EXTRA_ARGS` |
| `install:profile` | Persistent `nix profile add` into the user profile                    | `NIX_INSTALLABLE`, `NIX_EXTRA_ARGS`                |
| `uninstall`       | Uninstall Nix using the official Nix 2.34 steps                       | none                                               |
| `install:undo`    | Same as `uninstall`                                                   | none                                               |
| `upgrade`         | Upgrade Nix from `NIX_CHANNEL` via `nix-env`, then restart the daemon | `NIX_CHANNEL`                                      |
| `version`         | Show the installed Nix version                                        | none                                               |
| `features:enable` | Enable experimental features in `nix.conf`                            | `NIX_EXPERIMENTAL_FEATURES`, `NIX_CONF`            |
| `features:show`   | Show enabled features and the full Nix 2.34 catalog                   | `NIX_CONF`                                         |

## Variables

| Variable                        | Default                              | Description                                                                                                              |
| ------------------------------- | ------------------------------------ | ------------------------------------------------------------------------------------------------------------------------ |
| `NIX_INSTALL_URL`               | `https://nixos.org/nix/install`      | Unversioned official install script URL                                                                                  |
| `NIX_VERSION`                   | _(empty)_                            | Pin a release from `releases.nixos.org`; empty installs latest                                                           |
| `NIX_CHANNEL`                   | `nixpkgs-unstable`                   | nixpkgs channel used by `upgrade`                                                                                        |
| `NIX_CONF`                      | _(empty → `~/.config/nix/nix.conf`)_ | Path to `nix.conf` for `features:enable` and `features:show`                                                             |
| `NIX_EXPERIMENTAL_FEATURES`     | `nix-command flakes`                 | Features written by `features:enable`; pass `all` for every 2.34 flag                                                    |
| `NIX_NEEDED_FEATURES`           | `nix-command flakes`                 | Features `install:shell` and `install:profile` always enable                                                             |
| `NIX_EXPERIMENTAL_FEATURES_ALL` | every Nix 2.34 experimental feature  | Catalog used when `NIX_EXPERIMENTAL_FEATURES=all`                                                                        |
| `NIX_LOAD`                      | sources `nix-daemon.sh` or `nix.sh`  | Shell snippet that loads Nix into PATH                                                                                   |
| `NIX_INSTALLABLE`               | _(empty)_                            | Flake installable for `install:shell` and `install:profile` (e.g. `nixpkgs#hello`; bare `hello` becomes `nixpkgs#hello`) |
| `NIX_COMMAND`                   | _(empty)_                            | Optional command for `install:shell` (`nix shell --command`)                                                             |
| `NIX_EXTRA_ARGS`                | _(empty)_                            | Extra flags forwarded to `nix shell` / `nix profile add`                                                                 |

## Notes

**Linux (including WSL2):** `install` runs the 2.34 multi-user installer when
systemd is available:

```sh
curl --proto '=https' --tlsv1.2 -L https://nixos.org/nix/install | sh -s -- --daemon --yes
```

Without systemd it uses `--no-daemon` (single-user). WSL2 needs systemd for
the multi-user install.

**macOS:** `install` uses `--daemon` (multi-user is the only supported mode).

**Pinned version:**

```sh
task install NIX_VERSION=2.34.0
```

fetches `https://releases.nixos.org/nix/nix-2.34.0/install`.

**Packages:** `install:shell` and `install:profile` auto-enable `nix-command`
and `flakes` (no separate `features:enable` step). They also require
`NIX_INSTALLABLE`.

| Task              | Command                                               | Lifetime                        |
| ----------------- | ----------------------------------------------------- | ------------------------------- |
| `install:shell`   | `nix shell`                                           | Until the shell/command exits   |
| `install:profile` | `nix profile add` (`nix profile install` is an alias) | User profile (`~/.nix-profile`) |

```sh
task install:shell NIX_INSTALLABLE=nixpkgs#hello
task install:shell NIX_INSTALLABLE=hello NIX_COMMAND=hello
task install:profile NIX_INSTALLABLE=nixpkgs#jq
```

A value without `#` or a path prefix is expanded to `nixpkgs#<name>`.

**Upgrade** installs `nix` and `cacert` from `NIX_CHANNEL`, then restarts
`nix-daemon` on multi-user Linux (`systemctl`) and macOS (`launchctl`):

```sh
task upgrade
task upgrade NIX_CHANNEL=nixpkgs-unstable
```

**Uninstall** follows the official 2.34 steps (stop the daemon, delete store
and profiles, remove build users; on macOS also delete the Nix Store APFS
volume). Confirmation is required; pass `--yes` to skip the prompt.

**Native Windows:** Nix is not supported. Use WSL2 and run the tasks from
inside WSL.

## Experimental features

Nix 2.34 gates unstable functionality behind flags in `nix.conf`:

```
experimental-features = nix-command flakes
```

See the [experimental features](https://nix.dev/manual/nix/2.34/development/experimental-features.html)
and [`experimental-features` setting](https://nix.dev/manual/nix/2.34/command-ref/conf-file.html#experimental-features)
manual pages. Flags can change or be removed in later releases.

`features:enable` writes that setting to `~/.config/nix/nix.conf` (or
`NIX_CONF`). It **merges** with any flags already present.

```sh
task features:enable
task features:enable NIX_EXPERIMENTAL_FEATURES="nix-command flakes pipe-operators"
task features:enable NIX_EXPERIMENTAL_FEATURES=all
task features:enable NIX_CONF=/etc/nix/nix.conf
task features:show
```

Default is `nix-command flakes` — the usual pair for `nix build`, `nix run`,
and `nix flake`. `flakes` always enables `fetch-tree` as well.

| Feature | Description |
| ------------------------- | ------------------------------------------------------------------------- | ------- | ------------------------------- |
| `auto-allocate-uids` | Automatically pick UIDs for builds instead of creating `nixbld*` accounts |
| `blake3-hashes` | Support for BLAKE3 hashes |
| `ca-derivations` | Content-addressed derivations; skip rebuilds when outputs do not change |
| `cgroups` | Execute builds inside cgroups |
| `configurable-impure-env` | Allow the `impure-env` setting |
| `daemon-trust-override` | Force trusting or not trusting `nix-daemon` clients |
| `dynamic-derivations` | Text-hashed `.drv` outputs and dependencies on derivation outputs |
| `external-builders` | External builders / sandbox providers |
| `fetch-closure` | `builtins.fetchClosure` |
| `fetch-tree` | `builtins.fetchTree` (also enabled by `flakes`) |
| `flakes` | Flakes (`nix flake`) |
| `git-hashing` | Content-addressed store objects hashed with Git's hashing algorithm |
| `impure-derivations` | Derivations with `__impure` that may produce different outputs each build |
| `local-overlay-store` | Local overlay store |
| `mounted-ssh-store` | Mounted SSH store |
| `nix-command` | New `nix` subcommands (`nix build`, `nix run`, `nix flake`, …) |
| `parse-toml-timestamps` | Parse timestamps in `builtins.fromTOML` |
| `pipe-operators` | `                                                                         | >`and`< | ` operators in the Nix language |
| `read-only-local-store` | `read-only` parameter in local store URIs |
| `recursive-nix` | Allow derivation builders to call Nix |
| `verified-fetches` | Verify git commit signatures in `builtins.fetchGit` |

## Security Notes

**Install script**: `install` fetches the official Nix install script over
HTTPS and pipes it into `sh`. This is the method in the Nix 2.34 manual. It
relies on HTTPS transport for integrity — no additional checksum verification
is performed. Review the script at `NIX_INSTALL_URL` before running in
security-sensitive environments.
