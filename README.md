# MySetup

MySetup is a personal, data-driven Windows 11 workstation manager. It installs a curated package selection with exact WinGet IDs and applies intentionally portable configuration through a private, checksum-verified Chezmoi binary. Linux builds provide catalog validation and the controlled capture workflow only.

## Install

Released builds are installed without applying a profile:

```powershell
winget install -e --id reggieaalbios.MySetup
mysetup
```

The first-run TUI opens on an empty Custom profile. `Minimal`, `Developer`, and `Full` are editable presets; package and config selections remain independent.

## Commands

```text
mysetup                         interactive selector
mysetup status [--json]         ownership and drift state
mysetup plan [selection flags]  no-write plan
mysetup apply [selection flags] plan, confirm, apply
mysetup sync                    validate one protected-main snapshot, preview, apply
mysetup remove COMPONENT...     owned removal with drift protection
mysetup repair                  diagnose an interrupted transaction
mysetup doctor                  dependency and catalog checks
mysetup catalog validate        offline catalog/integrity validation
mysetup capture                 Linux-only maintainer capture
mysetup update                  upgrade the engine through WinGet
```

Selection flags are `--profile`, `--packages`, `--configs`, `--source`, `--yes`, and `--json`. State is stored under `%LOCALAPPDATA%\mysetup`. MySetup never captures or manages browser profiles, authentication, credentials, histories, caches, databases, generated state, or project data.

## Development

```bash
scripts/bootstrap.sh
go run scripts/update-integrity.go
scripts/verify.sh
```

All changes use feature branches and pull requests. Configure GitHub branch protection on `main` to require the `verify` check and disallow direct pushes. A successful Linux/CI run is not Windows acceptance: validate releases on both a fresh Windows 11 VM and an existing-user VM using [the acceptance checklist](docs/windows-acceptance.md).

## Scope

v1 provisions native Windows 11 only. macOS, WSL provisioning, Linux package installation, debloating, generic Windows tweaks, credentials, browser profiles, and the separate `Win11-window-tilling` project are out of scope.
