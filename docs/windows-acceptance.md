# Windows 11 acceptance

Run this checklist on both a fresh Windows 11 VM and an existing-user VM before declaring v1 complete.

- Install the manager through the local/review `reggieaalbios.MySetup` WinGet manifest; confirm installation changes no workstation packages or configs.
- Confirm first run is Custom with no selections.
- Apply a config without its parent package and confirm the warning and config behavior.
- Apply Minimal, Developer, Full, and a custom selection.
- Verify Micro wrapping, portable Git behavior and local identity prompt, PowerShell 7 with Starship, Windows-native Yazi openers, VS Code intentional files, and Windows Terminal dynamic profiles.
- Confirm GitHub authentication, Brave profiles, credentials, histories, caches, databases, and generated state remain untouched.
- Exercise a pre-existing config overwrite and injected mid-config failure; confirm originals are restored only for the failed transaction and successful transactions retain no backup.
- Interrupt an apply, run `repair`, and finish it. Confirm earlier successful package installs remain recorded.
- Modify a managed config and verify drift status and default preservation during remove.
- Verify only MySetup-owned packages uninstall and shared required dependencies remain protected.
- Uninstall the MySetup executable and confirm managed apps, configs, and state remain.
- Merge a test catalog PR into protected `main`; verify `sync` stages exactly one commit, validates the complete snapshot, previews the delta, and applies only after confirmation.
