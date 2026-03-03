#Requires -Version 5.1
<#
.SYNOPSIS
    Claude Agentic Relay installer for Windows.
.DESCRIPTION
    Downloads the prebuilt binary, installs the /relay skill,
    sets up auto-start via Scheduled Task, and configures projects.
.PARAMETER Port
    Relay port (default: 8090)
.PARAMETER SkipProjects
    Skip project scanning
.PARAMETER NoService
    Don't create auto-start scheduled task
.PARAMETER Uninstall
    Remove relay, service, and skill
.EXAMPLE
    irm https://raw.githubusercontent.com/Synergix-lab/claude-agentic-relay/main/install.ps1 | iex
    .\install.ps1 -Port 9000 -SkipProjects
    .\install.ps1 -Uninstall
#>
param(
    [int]$Port = 8090,
    [switch]$SkipProjects,
    [switch]$NoService,
    [switch]$Uninstall
)

$ErrorActionPreference = "Stop"

$Repo = "Synergix-lab/claude-agentic-relay"
$BinaryName = "agent-relay.exe"
$TaskName = "AgentRelay"
$InstallDir = Join-Path $env:LOCALAPPDATA "AgentRelay"
$BinPath = Join-Path $InstallDir $BinaryName
$SkillDir = Join-Path $env:USERPROFILE ".claude\commands"
$SkillPath = Join-Path $SkillDir "relay.md"
$DataDir = Join-Path $env:USERPROFILE ".agent-relay"

# ── Helpers ──────────────────────────────────────────────────────────────────

function Write-Step($num, $msg) {
    Write-Host ""
    Write-Host "[$num/5] $msg" -ForegroundColor Magenta
}

function Write-Ok($msg) {
    Write-Host "[OK] $msg" -ForegroundColor Green
}

function Write-Warn($msg) {
    Write-Host "[!] $msg" -ForegroundColor Yellow
}

function Write-Err($msg) {
    Write-Host "[X] $msg" -ForegroundColor Red
}

function Write-Info($msg) {
    Write-Host ":: $msg" -ForegroundColor Cyan
}

function Get-LatestVersion {
    try {
        $release = Invoke-RestMethod "https://api.github.com/repos/$Repo/releases/latest" -ErrorAction Stop
        return $release.tag_name
    } catch {
        return $null
    }
}

function Get-AgentName($dirName) {
    $lower = $dirName.ToLower()
    switch -Wildcard ($lower) {
        "*api*"       { return "backend" }
        "*backend*"   { return "backend" }
        "*server*"    { return "backend" }
        "*front*"     { return "frontend" }
        "*web*"       { return "frontend" }
        "*dashboard*" { return "frontend" }
        "*ui*"        { return "frontend" }
        "*infra*"     { return "infra" }
        "*deploy*"    { return "infra" }
        "*ops*"       { return "infra" }
        "*mobile*"    { return "mobile" }
        "*ios*"       { return "mobile" }
        "*android*"   { return "mobile" }
        "*docs*"      { return "docs" }
        "*test*"      { return "qa" }
        default       { return ($dirName.ToLower() -replace '[^a-z0-9]', '-' -replace '-+', '-' -replace '^-|-$', '') }
    }
}

# ── Banner ───────────────────────────────────────────────────────────────────

function Show-Banner {
    Write-Host ""
    Write-Host "  +=======================================+" -ForegroundColor Cyan
    Write-Host "  |   Claude Agentic Relay - Installer    |" -ForegroundColor Cyan
    Write-Host "  +=======================================+" -ForegroundColor Cyan
    Write-Host ""
    Write-Info "Platform: windows/amd64"
    Write-Info "Binary:   $BinPath"
    Write-Info "Port:     $Port"
    Write-Host ""
}

# ── Uninstall ────────────────────────────────────────────────────────────────

function Invoke-Uninstall {
    Write-Host ""
    Write-Host "  Uninstalling Claude Agentic Relay" -ForegroundColor Red
    Write-Host ""

    # Stop and remove scheduled task
    $task = Get-ScheduledTask -TaskName $TaskName -ErrorAction SilentlyContinue
    if ($task) {
        Write-Info "Removing scheduled task..."
        Stop-ScheduledTask -TaskName $TaskName -ErrorAction SilentlyContinue
        Unregister-ScheduledTask -TaskName $TaskName -Confirm:$false
        Write-Ok "Removed scheduled task"
    }

    # Stop running process
    Get-Process -Name "agent-relay" -ErrorAction SilentlyContinue | Stop-Process -Force

    # Remove binary
    if (Test-Path $BinPath) {
        Remove-Item $BinPath -Force
        Write-Ok "Removed binary"
    }

    # Remove install dir if empty
    if ((Test-Path $InstallDir) -and (!(Get-ChildItem $InstallDir))) {
        Remove-Item $InstallDir -Force
    }

    # Remove skill
    if (Test-Path $SkillPath) {
        Remove-Item $SkillPath -Force
        Write-Ok "Removed /relay skill"
    }

    # Data directory
    if (Test-Path $DataDir) {
        Write-Warn "Data directory exists: $DataDir"
        $answer = Read-Host "  Delete relay data (messages, agents)? [y/N]"
        if ($answer -eq "y") {
            Remove-Item $DataDir -Recurse -Force
            Write-Ok "Removed data directory"
        } else {
            Write-Info "Kept data directory"
        }
    }

    Write-Host ""
    Write-Ok "Uninstall complete"
    exit 0
}

# ── Step 1: Install binary ──────────────────────────────────────────────────

function Install-Binary {
    Write-Step 1 "Installing binary"

    New-Item -ItemType Directory -Path $InstallDir -Force | Out-Null

    # Check existing
    if (Test-Path $BinPath) {
        $ver = & $BinPath --version 2>$null
        Write-Warn "Existing install detected: $ver"
        $answer = Read-Host "  Upgrade? [Y/n]"
        if ($answer -eq "n") {
            Write-Info "Skipping binary install"
            return
        }
    }

    $version = Get-LatestVersion
    if (-not $version) {
        Write-Err "No releases found. Visit https://github.com/$Repo/releases"
        exit 1
    }

    $archiveName = "agent-relay-windows-amd64.zip"
    $url = "https://github.com/$Repo/releases/download/$version/$archiveName"

    Write-Info "Downloading $version..."
    $tmpDir = Join-Path $env:TEMP "agent-relay-install"
    New-Item -ItemType Directory -Path $tmpDir -Force | Out-Null

    try {
        Invoke-WebRequest -Uri $url -OutFile "$tmpDir\archive.zip" -UseBasicParsing
    } catch {
        Write-Err "Download failed: $_"
        Write-Err "Check https://github.com/$Repo/releases"
        exit 1
    }

    Expand-Archive -Path "$tmpDir\archive.zip" -DestinationPath $tmpDir -Force
    Copy-Item "$tmpDir\agent-relay.exe" $BinPath -Force
    Remove-Item $tmpDir -Recurse -Force

    Write-Ok "Installed $version"

    # Add to PATH if not already there
    $userPath = [Environment]::GetEnvironmentVariable("Path", "User")
    if ($userPath -notlike "*$InstallDir*") {
        [Environment]::SetEnvironmentVariable("Path", "$userPath;$InstallDir", "User")
        $env:Path = "$env:Path;$InstallDir"
        Write-Ok "Added $InstallDir to PATH"
    }
}

# ── Step 2: Install service ─────────────────────────────────────────────────

function Install-Service {
    Write-Step 2 "Setting up auto-start"

    if ($NoService) {
        Write-Info "Skipped (-NoService)"
        return
    }

    # Remove existing task
    $existing = Get-ScheduledTask -TaskName $TaskName -ErrorAction SilentlyContinue
    if ($existing) {
        Stop-ScheduledTask -TaskName $TaskName -ErrorAction SilentlyContinue
        Unregister-ScheduledTask -TaskName $TaskName -Confirm:$false
    }

    $action = New-ScheduledTaskAction -Execute $BinPath
    $trigger = New-ScheduledTaskTrigger -AtLogOn -User $env:USERNAME
    $settings = New-ScheduledTaskSettingsSet -AllowStartIfOnBatteries -DontStopIfGoingOnBatteries -RestartCount 3 -RestartInterval (New-TimeSpan -Seconds 10)

    Register-ScheduledTask -TaskName $TaskName -Action $action -Trigger $trigger -Settings $settings -Description "Claude Agentic Relay" | Out-Null

    # Start the task now
    Start-ScheduledTask -TaskName $TaskName -ErrorAction SilentlyContinue

    Write-Ok "Installed scheduled task (starts on login, auto-restarts)"
}

# ── Step 3: Install skill ───────────────────────────────────────────────────

function Install-Skill {
    Write-Step 3 "Installing /relay skill"

    New-Item -ItemType Directory -Path $SkillDir -Force | Out-Null

    try {
        Invoke-WebRequest -Uri "https://raw.githubusercontent.com/$Repo/main/skill/relay.md" -OutFile $SkillPath -UseBasicParsing
    } catch {
        Write-Warn "Couldn't download skill file, creating from template"
        @"
You are an inter-agent communication assistant using the Agent Relay MCP server.

## Your Identity

Extract your agent name from the MCP server URL in the project's ``.mcp.json`` file (the ``?agent=`` query parameter). If you can't determine it, ask the user.

## Commands

Parse the user's arguments from ``$ARGUMENTS``:

- **No arguments** or **inbox**: Check inbox for unread messages
- **send <agent> <message>**: Send a message to another agent
- **agents**: List all registered agents
- **thread <message_id>**: View a complete conversation thread
- **read**: Mark all unread messages as read
- **conversations**: List your conversations with unread counts
- **create <title> <agent1> [agent2] ...**: Create a conversation
- **msg <conversation_id> <message>**: Send to a conversation
- **invite <conversation_id> <agent>**: Invite agent to conversation
- **talk**: Enter conversation mode (proactive loop)

## Behavior

### On first invocation
1. Call ``register_agent`` with your agent name, role, description, and optionally ``reports_to`` (org hierarchy)
2. Then execute the requested command
"@ | Set-Content $SkillPath
    }

    Write-Ok "Installed /relay command at $SkillPath"
}

# ── Step 4: Scan and configure projects ──────────────────────────────────────

function Find-AndConfigureProjects {
    Write-Step 4 "Scanning for Claude Code projects"

    if ($SkipProjects) {
        Write-Info "Skipped (-SkipProjects)"
        return
    }

    Write-Info "Looking for projects with .mcp.json or CLAUDE.md..."

    $projects = @()
    $searchPaths = @(
        (Join-Path $env:USERPROFILE "Documents"),
        (Join-Path $env:USERPROFILE "Projects"),
        (Join-Path $env:USERPROFILE "repos"),
        (Join-Path $env:USERPROFILE "dev"),
        (Join-Path $env:USERPROFILE "src"),
        (Join-Path $env:USERPROFILE "code"),
        $env:USERPROFILE
    )

    foreach ($searchPath in $searchPaths) {
        if (-not (Test-Path $searchPath)) { continue }
        $found = Get-ChildItem -Path $searchPath -Depth 2 -Include "CLAUDE.md", ".mcp.json" -Recurse -ErrorAction SilentlyContinue
        foreach ($file in $found) {
            $dir = $file.DirectoryName
            if ($dir -match "node_modules|vendor|\.git|agent-relay") { continue }
            if ($projects -notcontains $dir) {
                $projects += $dir
            }
        }
    }

    if ($projects.Count -eq 0) {
        Write-Info "No Claude Code projects found"
        Write-Info "Manually add to .mcp.json: {`"mcpServers`":{`"agent-relay`":{`"type`":`"http`",`"url`":`"http://localhost:${Port}/mcp?agent=NAME`"}}}"
        return
    }

    Write-Host ""
    Write-Info "Found $($projects.Count) project(s):"
    Write-Host ""

    $agentNames = @()
    for ($i = 0; $i -lt $projects.Count; $i++) {
        $name = Get-AgentName (Split-Path $projects[$i] -Leaf)
        $agentNames += $name
        $configured = ""
        $mcpFile = Join-Path $projects[$i] ".mcp.json"
        if ((Test-Path $mcpFile) -and (Get-Content $mcpFile -Raw) -match "agent-relay") {
            $configured = " (already configured)"
        }
        Write-Host ("  {0,2}) {1,-40} -> agent: {2}{3}" -f ($i + 1), $projects[$i].Replace($env:USERPROFILE, "~"), $name, $configured)
    }

    Write-Host ""
    $choice = Read-Host "  Configure which projects? (a=all / comma-separated numbers / n=none)"

    switch ($choice.ToLower()) {
        "n" { Write-Info "Skipped"; return }
        { $_ -eq "a" -or $_ -eq "" } { }
        default {
            $indices = $choice -split "," | ForEach-Object { [int]$_.Trim() - 1 } | Where-Object { $_ -ge 0 -and $_ -lt $projects.Count }
            $projects = $projects[$indices]
            $agentNames = $agentNames[$indices]
        }
    }

    Write-Host ""
    for ($i = 0; $i -lt $projects.Count; $i++) {
        Set-ProjectConfig $projects[$i] $agentNames[$i]
    }
}

function Set-ProjectConfig($projectDir, $agentName) {
    $mcpPath = Join-Path $projectDir ".mcp.json"
    $relayEntry = @{ type = "http"; url = "http://localhost:$Port/mcp?agent=$agentName" }

    if (Test-Path $mcpPath) {
        $content = Get-Content $mcpPath -Raw
        if ($content -match "agent-relay") {
            Write-Ok "$($projectDir.Replace($env:USERPROFILE, '~')) - already configured"
            return
        }
        try {
            $data = $content | ConvertFrom-Json
            if (-not $data.mcpServers) {
                $data | Add-Member -NotePropertyName "mcpServers" -NotePropertyValue @{} -Force
            }
            $data.mcpServers | Add-Member -NotePropertyName "agent-relay" -NotePropertyValue $relayEntry -Force
            $data | ConvertTo-Json -Depth 10 | Set-Content $mcpPath
        } catch {
            Write-Warn "Could not parse $mcpPath - skipping"
            return
        }
    } else {
        @{ mcpServers = @{ "agent-relay" = $relayEntry } } | ConvertTo-Json -Depth 10 | Set-Content $mcpPath
    }

    Write-Ok "$($projectDir.Replace($env:USERPROFILE, '~')) -> $agentName"
}

# ── Step 5: Verify ──────────────────────────────────────────────────────────

function Test-Installation {
    Write-Step 5 "Verifying installation"

    if (Test-Path $BinPath) {
        $ver = & $BinPath --version 2>$null
        Write-Ok "Binary: $ver"
    } else {
        Write-Err "Binary not found at $BinPath"
    }

    if (Test-Path $SkillPath) {
        Write-Ok "Skill: /relay command installed"
    } else {
        Write-Warn "Skill: not found"
    }

    if (-not $NoService) {
        Start-Sleep -Seconds 2
        try {
            $null = Invoke-WebRequest -Uri "http://localhost:$Port/mcp" -UseBasicParsing -TimeoutSec 3
            Write-Ok "Service: relay running on port $Port"
        } catch {
            Write-Warn "Service: relay not responding yet (may need a moment)"
        }
    }
}

# ── Summary ──────────────────────────────────────────────────────────────────

function Show-Summary {
    Write-Host ""
    Write-Host "  +=======================================+" -ForegroundColor Green
    Write-Host "  |      Installation complete!            |" -ForegroundColor Green
    Write-Host "  +=======================================+" -ForegroundColor Green
    Write-Host ""
    Write-Info "The relay is running on http://localhost:$Port"
    Write-Host ""
    Write-Info "Next steps:"
    Write-Host "  1. Open Claude Code in any configured project"
    Write-Host "  2. Use /relay to check your inbox"
    Write-Host "  3. Use /relay send <agent> <message> to talk to another agent"
    Write-Host ""
    Write-Info "Uninstall: .\install.ps1 -Uninstall"
    Write-Host ""
}

# ── Main ─────────────────────────────────────────────────────────────────────

if ($Uninstall) {
    Invoke-Uninstall
}

Show-Banner
Install-Binary
Install-Service
Install-Skill
Find-AndConfigureProjects
Test-Installation
Show-Summary
