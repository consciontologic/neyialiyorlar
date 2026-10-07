# Map neyialiyorlar.local -> loopback. Run in an elevated PowerShell.
$h = "$env:WINDIR\System32\drivers\etc\hosts"
$host_ = "neyialiyorlar.local"

if (Select-String -Path $h -Pattern "\s$([regex]::Escape($host_))(\s|$)" -Quiet) {
  Write-Host "ℹ️  already mapped"
  return
}

Copy-Item $h "$h.neyialiyorlar.bak"            # reversible backup
Add-Content -Path $h -Value "`n127.0.0.1`t$host_"
Write-Host "✅ added 127.0.0.1 $host_"
# mkcert on Windows: `choco install mkcert; mkcert -install` then run make-certs equivalent.
