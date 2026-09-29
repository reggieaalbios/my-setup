#!/usr/bin/env bash
set -euo pipefail

repo_root="$(git rev-parse --show-toplevel)"
git -C "$repo_root" config core.hooksPath .githooks

skill_parent="${CODEX_HOME:-$HOME/.codex}/skills"
skill_target="$skill_parent/mysetup-maintainer"
mkdir -p "$skill_parent"
if [[ -e "$skill_target" && ! -L "$skill_target" ]]; then
  printf 'Refusing to replace existing non-symlink skill: %s\n' "$skill_target" >&2
  exit 1
fi
ln -sfn "$repo_root/skills/mysetup-maintainer" "$skill_target"
printf 'Configured hooks and installed %s\n' "$skill_target"
