# uv Taskfile Public Tasks

## What is this Taskfile?

A Taskfile for running uv Python project operations — virtual environments,
package installation, script execution, and isolated tool management. The uv
binary is installed through `nix:install:profile`.

## Usage

### Standalone

```sh
task nix:install:profile NIX_INSTALLABLE=nixpkgs#uv
task -t taskfiles/uv/Taskfile.yml venv
task -t taskfiles/uv/Taskfile.yml tool:install UV_TOOL=yamllint
```

### Included

```yaml
includes:
  uv: ./taskfiles/uv/Taskfile.yml
```

Then run:

```sh
task uv:venv
task uv:tool:install UV_TOOL=ruff
```

Pin a revision by overriding the installable, for example
`UV_NIX_INSTALLABLE=github:NixOS/nixpkgs/<rev>#uv`.

## Public Tasks

| Task             | Description                                        | Key variables                         |
| ---------------- | -------------------------------------------------- | ------------------------------------- |
| `python:install` | Install a Python version via uv                    | `PYTHON_VERSION`                      |
| `venv`           | Create a virtual environment                       | `UV_VENV`, `UV_EXTRA_ARGS`            |
| `pip:install`    | Install packages from a requirements file          | `UV_REQUIREMENTS`, `UV_EXTRA_ARGS`    |
| `run`            | Run a script or command via uv                     | `UV_FILE`, `UV_ARGS`, `UV_EXTRA_ARGS` |
| `tool:install`   | Install a Python tool into an isolated environment | `UV_TOOL`, `UV_EXTRA_ARGS`            |
| `tool:upgrade`   | Upgrade an installed uv tool                       | `UV_TOOL`, `UV_EXTRA_ARGS`            |
| `install`        | Install uv via Nix (Unix) or WinGet (Windows)      | `UV_NIX_INSTALLABLE`                  |
| `version`        | Show the active uv version                         | —                                     |

## Variables

| Variable                | Default                                                | Description                                                                                         |
| ----------------------- | ------------------------------------------------------ | --------------------------------------------------------------------------------------------------- |
| `UV_NIX_INSTALLABLE`    | `nixpkgs#uv`                                           | Flake installable passed to `nix:install:profile`                                                   |
| `UV_WINGET_INSTALLABLE` | `astral-sh.uv`                                         | WinGet package ID for `winget:install:package`                                                      |
| `UV_LOAD`               | reloads User Path; prepends `%USERPROFILE%\.local\bin` | PowerShell snippet so `uv` and uv-installed tools from earlier in the same Task process are on PATH |
| `UV_VENV`               | `.venv`                                                | Virtual environment directory for `venv`                                                            |
| `UV_REQUIREMENTS`       | `requirements.txt`                                     | Requirements file for `pip:install`                                                                 |
| `UV_FILE`               | _(empty)_                                              | Script path; required by `run`                                                                      |
| `UV_ARGS`               | _(empty)_                                              | Positional arguments forwarded to the script in `run`                                               |
| `UV_EXTRA_ARGS`         | _(empty)_                                              | Extra flags forwarded to the underlying uv command                                                  |
| `PYTHON_VERSION`        | _(empty)_                                              | Python version to install; required by `python:install`                                             |
| `UV_TOOL`               | _(empty)_                                              | Tool name; required by `tool:install` and `tool:upgrade`                                            |

## Notes

- Install uses Nix on Linux and macOS (`UV_NIX_INSTALLABLE`) and WinGet on Windows (`UV_WINGET_INSTALLABLE`, default `astral-sh.uv`).

- Remaining uv tasks auto-install the uv binary if it is missing.
- **`tool:install`** creates an isolated environment for each tool so their dependencies never conflict with your project. The tool's binary is shimmed into `~/.local/bin` (Unix) or `%USERPROFILE%\.local\bin` (Windows).
- On Windows, `UV_LOAD` reloads User Path and prepends `%USERPROFILE%\.local\bin` so uv-installed tools from earlier in the same Task process are on PATH.
