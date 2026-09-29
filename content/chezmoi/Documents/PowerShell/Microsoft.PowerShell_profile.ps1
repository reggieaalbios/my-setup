# MySetup-managed PowerShell 7 profile. Authentication and machine-specific paths are intentionally excluded.
if (Get-Command starship -ErrorAction SilentlyContinue) {
    Invoke-Expression (&starship init powershell)
}
if (Get-Command zoxide -ErrorAction SilentlyContinue) {
    Invoke-Expression (&zoxide init powershell)
}
if (Get-Command fzf -ErrorAction SilentlyContinue) {
    Set-PSReadLineKeyHandler -Key Tab -ScriptBlock { Invoke-FzfTabCompletion }
}
function .. { Set-Location .. }
function ... { Set-Location ../.. }
function gst { git status --short --branch @args }
function gco { git checkout @args }
Set-Alias -Name edit -Value micro -ErrorAction SilentlyContinue
