# install.ps1 puts xscapes on a Windows machine with one command, with or
# without Go:
#
#     irm https://donlucasx.github.io/xscapes/install.ps1 | iex
#
# It downloads the release binary for this processor (amd64 or arm64),
# checks it against the release's checksums.txt, installs it to
# %LOCALAPPDATA%\Programs\xscapes, and adds that directory to the USER's PATH
# (no administrator rights, nothing machine-wide). A change to PATH reaches
# new terminals only, so the next steps are printed with a full path that
# works in THIS window.
#
# Needs Windows 10 version 1809 (build 17763) or later: the host runs the agent
# on a pseudoconsole (ConPTY), which earlier builds do not have.
#
# Overrides (environment variables, so they survive `irm | iex`):
#   XSCAPES_INSTALL_DIR    where the binary goes
#   XSCAPES_DOWNLOAD_BASE  where the assets are (default: latest GitHub release)
#   XSCAPES_VERSION        a release tag to install instead of the latest
#   XSCAPES_SKIP_VERIFY=1  install without checking the download's SHA-256
#   XSCAPES_NO_PATH=1      do not touch the PATH
#
# To remove it again:
#   Remove-Item "$env:LOCALAPPDATA\Programs\xscapes" -Recurse
# and delete that directory from your user PATH (Settings > System > About >
# Advanced system settings > Environment Variables). `xscapes uninstall
# claude|kimi|hermes` puts the agents' settings back first.
#
# ASCII only on purpose: Windows PowerShell 5.1 reads a script without a BOM
# in the ANSI code page.

$ErrorActionPreference = 'Stop'
$ProgressPreference = 'SilentlyContinue'   # Invoke-WebRequest is 10x slower with the bar

function Say($m)  { Write-Host $m }
function Fail($m) { Write-Host "xscapes: $m" -ForegroundColor Red; throw "xscapes install failed" }

# A script run through `iex` must not close the user's window with `exit`, so
# everything is one function that returns.
function Install-Xscapes {
    $repo = 'donlucasx/xscapes'

    # Windows 10 1809 or later.
    $build = [Environment]::OSVersion.Version.Build
    if ($build -lt 17763) {
        Fail "Windows build $build is too old; the scape needs 17763 (Windows 10 1809) or later for ConPTY."
    }

    # TLS 1.2 for Windows PowerShell 5.1, whose default can be older.
    try { [Net.ServicePointManager]::SecurityProtocol = [Net.ServicePointManager]::SecurityProtocol -bor [Net.SecurityProtocolType]::Tls12 } catch { }

    # The real processor, not the process's view of it (a 32-bit PowerShell on
    # a 64-bit machine reports x86 in PROCESSOR_ARCHITECTURE).
    $archRaw = $env:PROCESSOR_ARCHITEW6432
    if (-not $archRaw) { $archRaw = $env:PROCESSOR_ARCHITECTURE }
    switch -Regex ($archRaw) {
        '^(AMD64|x86_64)$' { $arch = 'amd64' }
        '^ARM64$'          { $arch = 'arm64' }
        default            { Fail "no build for the $archRaw processor (builds: amd64, arm64)." }
    }
    $asset = "xscapes-windows-$arch.exe"

    if ($env:XSCAPES_DOWNLOAD_BASE) {
        $base = $env:XSCAPES_DOWNLOAD_BASE.TrimEnd('/')
    } elseif ($env:XSCAPES_VERSION) {
        $base = "https://github.com/$repo/releases/download/$($env:XSCAPES_VERSION)"
    } else {
        $base = "https://github.com/$repo/releases/latest/download"
    }

    if ($env:XSCAPES_INSTALL_DIR) {
        $dir = $env:XSCAPES_INSTALL_DIR
    } else {
        $dir = Join-Path $env:LOCALAPPDATA 'Programs\xscapes'
    }
    New-Item -ItemType Directory -Force -Path $dir | Out-Null

    $tmp = Join-Path $dir ".xscapes.$([guid]::NewGuid().ToString('N')).tmp"
    $dest = Join-Path $dir 'xscapes.exe'
    $how = $asset
    try {
        $downloaded = $false
        $dlError = ''
        try {
            Invoke-WebRequest -UseBasicParsing -Uri "$base/$asset" -OutFile $tmp
            $downloaded = $true
        } catch {
            $dlError = $_.Exception.Message
        }

        if ($downloaded) {
            # Verify against checksums.txt (site/release.sh writes it). A
            # mismatch is fatal; a release without the file installs with a
            # warning, because the first releases never carried one.
            if ($env:XSCAPES_SKIP_VERIFY -ne '1') {
                $sums = $null
                try { $sums = (Invoke-WebRequest -UseBasicParsing -Uri "$base/checksums.txt").Content } catch { }
                if ($sums -is [byte[]]) { $sums = [Text.Encoding]::ASCII.GetString($sums) }
                if ($sums) {
                    $want = $null
                    foreach ($line in ($sums -split "`r?`n")) {
                        $parts = $line.Trim() -split '\s+'
                        if ($parts.Count -ge 2 -and ($parts[1].TrimStart('*') -eq $asset)) { $want = $parts[0].ToLower() }
                    }
                    if (-not $want) { Fail "checksums.txt has no entry for $asset." }
                    $got = (Get-FileHash -Algorithm SHA256 -Path $tmp).Hash.ToLower()
                    if ($got -ne $want) { Fail "checksum mismatch for $asset (expected $want, got $got); nothing was installed." }
                } else {
                    Say 'xscapes: this release has no checksums.txt; the download was not verified.'
                }
            }
        } elseif (Get-Command go -ErrorAction SilentlyContinue) {
            Say "no release binary for windows/$arch ($dlError); building one with go install"
            $env:GOBIN = $dir
            & go install "github.com/$repo@latest"
            if ($LASTEXITCODE -ne 0) { Fail 'go install failed.' }
            $how = 'go install'
            $tmp = $null
        } else {
            Fail "no release binary for windows/$arch ($dlError) and no Go to build one.`n         Install Go (winget install GoLang.Go, or https://go.dev/dl) and run this again."
        }

        if ($tmp) {
            # The new file goes in beside the old one and is swapped in by a
            # rename. A running xscapes.exe cannot be overwritten, but it CAN be
            # renamed, so the old one is moved aside and removed on a later run.
            Get-ChildItem -Path $dir -Filter 'xscapes.exe.old-*' -ErrorAction SilentlyContinue |
                ForEach-Object { Remove-Item -Force $_.FullName -ErrorAction SilentlyContinue }
            if (Test-Path $dest) {
                Move-Item -Force $dest "$dest.old-$([guid]::NewGuid().ToString('N').Substring(0,8))"
            }
            Move-Item -Force $tmp $dest
            $tmp = $null
        }
    } finally {
        if ($tmp -and (Test-Path $tmp)) { Remove-Item -Force $tmp -ErrorAction SilentlyContinue }
    }

    # It must run: `-h` exits 0 and prints the help.
    & $dest -h *> $null
    if ($LASTEXITCODE -ne 0) { Fail "the installed binary does not run: $dest" }

    # PATH: the user's, persistently, once.
    $onPath = $false
    $userPath = [Environment]::GetEnvironmentVariable('Path', 'User')
    foreach ($p in (($userPath, $env:Path) -join ';') -split ';') {
        if ($p -and ($p.TrimEnd('\') -ieq $dir.TrimEnd('\'))) { $onPath = $true }
    }
    $pathNote = ''
    if (-not $onPath) {
        if ($env:XSCAPES_NO_PATH -eq '1') {
            $pathNote = "Add $dir to your PATH so new terminals find it by name."
        } else {
            if ([string]::IsNullOrEmpty($userPath)) { $new = $dir } else { $new = "$($userPath.TrimEnd(';'));$dir" }
            [Environment]::SetEnvironmentVariable('Path', $new, 'User')
            $pathNote = "Every NEW terminal will find it by name ($dir was added to your user PATH)."
        }
    }
    # This window too, for the commands below.
    if ($env:Path -notlike "*$dir*") { $env:Path = "$env:Path;$dir" }

    Say ''
    Say "xscapes installed: $dest ($how)"
    if ($pathNote) { Say $pathNote }
    Say ''
    Say 'Next:'
    Say '  xscapes install claude --apply   # writes its hooks, after a backup'
    Say '  xscapes claude                   # the agent on top, the scape underneath'
    Say '  xscapes install kimi --apply; xscapes kimi          # Kimi Code CLI'
    Say '  xscapes install hermes --apply; xscapes hermes      # Hermes Agent'
    Say '  xscapes inside <any command>     # anything else'
    Say ''
    Say 'Use Windows Terminal (or any VT-capable console) for the best picture.'
}

Install-Xscapes
