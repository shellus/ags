$ErrorActionPreference = "Stop"

$repo = "shellus/ags"
$asset = "ags-windows-amd64.exe"
$base = "https://github.com/$repo/releases/latest/download"
$targetDir = if ($env:AGS_INSTALL_DIR) { $env:AGS_INSTALL_DIR } else { Join-Path $HOME "bin" }
$tmp = Join-Path ([System.IO.Path]::GetTempPath()) ("ags-install-" + [guid]::NewGuid())

New-Item -ItemType Directory -Force $tmp | Out-Null
try {
    Invoke-WebRequest "$base/$asset" -OutFile (Join-Path $tmp $asset)
    Invoke-WebRequest "$base/checksums.txt" -OutFile (Join-Path $tmp "checksums.txt")

    $expectedLine = Get-Content (Join-Path $tmp "checksums.txt") | Where-Object { $_ -match "\s+$([regex]::Escape($asset))$" } | Select-Object -First 1
    if (-not $expectedLine) { throw "checksums.txt does not contain $asset" }
    $expected = ($expectedLine -split "\s+")[0].ToLowerInvariant()
    $actual = (Get-FileHash (Join-Path $tmp $asset) -Algorithm SHA256).Hash.ToLowerInvariant()
    if ($actual -ne $expected) { throw "checksum mismatch for $asset" }

    New-Item -ItemType Directory -Force $targetDir | Out-Null
    Copy-Item -Force (Join-Path $tmp $asset) (Join-Path $targetDir "ags.exe")

    $userPath = [Environment]::GetEnvironmentVariable("Path", "User")
    $entries = @($userPath -split ";" | Where-Object { $_ })
    if ($entries -notcontains $targetDir) {
        $newPath = (@($entries) + $targetDir) -join ";"
        [Environment]::SetEnvironmentVariable("Path", $newPath, "User")
    }
    Write-Host "Installed AGS to $(Join-Path $targetDir 'ags.exe')"
    Write-Host "Open a new terminal before running ags."
}
finally {
    Remove-Item -Recurse -Force $tmp -ErrorAction SilentlyContinue
}
