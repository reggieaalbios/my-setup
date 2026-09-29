---
name: mysetup-maintainer
description: Maintain the owner's MySetup catalog and portable Windows configuration from an explicit capture report. Use only when invoked as $mysetup-maintainer inside the my-setup checkout.
---

# MySetup Maintainer

Process an explicit report produced by `mysetup capture`. The user's stated intent and the report are inputs, not blanket permission to capture the host.

Before editing, inspect the report and current repository state. Check proposed APIs, package identifiers, paths, and practices for deprecation, lack of support, unsafe behavior, or obsolete assumptions. Stop and explain a material problem before implementing it; use maintained alternatives while retaining intentional compatibility.

## Boundaries

- Verify WinGet identifiers and application config locations with authoritative current sources.
- Translate portable intent into native Windows 11 behavior. Never copy Linux-only openers, absolute paths, credential helpers, XDG/Kitty integration, or shell assumptions unchanged.
- Reject credentials, tokens, authentication files, cookies, sessions, histories, caches, logs, databases, backups, machine IDs, generated state, and broad directory copies.
- Keep package and config choices independent. Ordinary additions belong in catalog and Chezmoi data; edit Go only for a new provider or engine capability.
- Preserve the user's unrelated work and never merge a pull request.

## Workflow

1. Confirm the report is under `.mysetup/captures/` in this checkout and review every candidate path.
2. Create `agent/YYYY-MM-DD-short-slug` from an up-to-date protected `main`. Do not work directly on `main`.
3. Make the narrow catalog/config change. Keep secret and generated-state exclusions explicit.
4. Run `go run scripts/update-integrity.go`, then `scripts/verify.sh`.
5. Review the complete diff for sensitive data and unintended generated files.
6. Commit the coherent change. Run the configured pre-push checks, push the feature branch, and open a pull request with validation results and any Windows VM checks still required.
7. Stop after opening the pull request. Merging is always the user's decision.

If Codex or GitHub authentication is unavailable, preserve all local work and give the exact safe command the user can run next.
