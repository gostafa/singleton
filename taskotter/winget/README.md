# WinGet Taskfile Public Tasks

## What is this Taskfile?

A Taskfile for installing, upgrading, and uninstalling packages with
[WinGet](https://learn.microsoft.com/windows/package-manager/winget/), the
Windows Package Manager.

WinGet itself is bootstrapped on first use (App Installer registration, then
`Repair-WinGetPackageManager` if needed). Linux and macOS are not supported;
use the [`nix`](../nix/README.md) module there.

The rest of the store can install CLI tools on native Windows through
`winget:install:package`. Modules own a `{TOOL}_WINGET_INSTALLABLE` and depend
on that task instead of shipping their own Windows installers.

## Usage

### Standalone

```sh
task -t taskfiles/winget/Taskfile.yml install
task -t taskfiles/winget/Taskfile.yml install:package WINGET_INSTALLABLE=Git.Git
task -t taskfiles/winget/Taskfile.yml uninstall WINGET_INSTALLABLE=Git.Git
```

### Included

```yaml
includes:
  winget: ./taskfiles/winget/Taskfile.yml
```

Then run:

```sh
task winget:install
task winget:install:package WINGET_INSTALLABLE=Git.Git
task winget:upgrade WINGET_INSTALLABLE=Git.Git
```

### From another module

```yaml
includes:
  winget:
    taskfile: ../winget/Taskfile.yml

vars:
  JQ_WINGET_INSTALLABLE: '{{.JQ_WINGET_INSTALLABLE | default "jqlang.jq"}}'

tasks:
  install:
    cmds:
      - task: winget:install:package
        vars:
          WINGET_INSTALLABLE: '{{.JQ_WINGET_INSTALLABLE}}'
```

## Auto-install behaviour

`version`, `install:package`, `uninstall`, and `upgrade` install WinGet first
if it is missing.

`install:package`, `uninstall`, and `upgrade` `require` `WINGET_INSTALLABLE` and
fail the precondition if it is empty.

Installs of WinGet are **idempotent**: the internal install task skips work when
`winget` is already on PATH.

`install:package` treats an already-installed package as success and does not
upgrade it (`--no-upgrade`). Use `upgrade` to move to a newer release.

## Public Tasks

| Task              | Description                                    | Key variables                                                                                |
| ----------------- | ---------------------------------------------- | -------------------------------------------------------------------------------------------- |
| `install`         | Install WinGet on Windows if missing           | none                                                                                         |
| `install:package` | Persistent `winget install --id` for a package | `WINGET_INSTALLABLE`, `WINGET_VERSION`, `WINGET_SCOPE`, `WINGET_SOURCE`, `WINGET_EXTRA_ARGS` |
| `uninstall`       | `winget uninstall --id`                        | `WINGET_INSTALLABLE`, `WINGET_VERSION`, `WINGET_SCOPE`, `WINGET_SOURCE`, `WINGET_EXTRA_ARGS` |
| `install:undo`    | Same as `uninstall`                            | `WINGET_INSTALLABLE`                                                                         |
| `upgrade`         | `winget upgrade --id`                          | `WINGET_INSTALLABLE`, `WINGET_VERSION`, `WINGET_SCOPE`, `WINGET_SOURCE`, `WINGET_EXTRA_ARGS` |
| `version`         | Show the installed WinGet version              | none                                                                                         |

## Variables

| Variable             | Default                                                               | Description                                                                                                                                                           |
| -------------------- | --------------------------------------------------------------------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `WINGET_INSTALLABLE` | _(empty)_                                                             | WinGet package ID for `install:package`, `uninstall`, and `upgrade` (e.g. `Git.Git`)                                                                                  |
| `WINGET_VERSION`     | _(empty)_                                                             | Pin a package version (`winget --version`); empty installs latest                                                                                                     |
| `WINGET_SCOPE`       | _(empty)_                                                             | Optional `user` or `machine` scope                                                                                                                                    |
| `WINGET_SOURCE`      | `winget`                                                              | WinGet source; default avoids Microsoft Store agreement prompts                                                                                                       |
| `WINGET_EXTRA_ARGS`  | _(empty)_                                                             | Extra flags forwarded to `winget install` / `uninstall` / `upgrade`                                                                                                   |
| `WINGET_LOAD`        | reloads Machine/User Path; prepends WinGet Links; appends WindowsApps | PowerShell snippet so `winget` and packages installed earlier in the same Task process are on PATH. WindowsApps is last so Store aliases do not shadow real installs. |

## Notes

**Windows:** `install` registers App Installer when it is already present:

```powershell
Add-AppxPackage -RegisterByFamilyName -MainPackage Microsoft.DesktopAppInstaller_8wekyb3d8bbwe
```

If `winget` is still missing (Windows Sandbox, some CI images), it bootstraps
with `Install-Module Microsoft.WinGet.Client` and
`Repair-WinGetPackageManager`.

**Packages:** `install:package` is the Windows counterpart of
`nix:install:profile`. It always passes `--id`, `--exact`, `--silent`,
`--disable-interactivity`, `--accept-source-agreements`, and
`--accept-package-agreements`.

```sh
task install:package WINGET_INSTALLABLE=Git.Git
task install:package WINGET_INSTALLABLE=jqlang.jq WINGET_VERSION=1.7.1
task install:package WINGET_INSTALLABLE=GoLang.Go WINGET_SCOPE=user
```

**Upgrade** moves one package to a newer release (or to `WINGET_VERSION`):

```sh
task upgrade WINGET_INSTALLABLE=Git.Git
```

**Uninstall** removes the package. There is no confirmation prompt, so other
modules can call it from CI.

**Linux and macOS:** WinGet is not supported. Use the nix module.

## Security Notes

**Bootstrap**: `install` may download the `Microsoft.WinGet.Client` module from
the PowerShell Gallery and run `Repair-WinGetPackageManager` when App Installer
is missing. Review those steps before running in security-sensitive
environments. Package installs trust the configured WinGet source (default
`winget`).
