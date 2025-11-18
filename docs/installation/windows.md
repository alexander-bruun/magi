# Windows Installation Guide

This guide covers installing Magi on Windows as a native application or Windows Service.

## Prerequisites

- Windows 10/11 or Windows Server 2016+
- 512MB+ available RAM
- Storage for manga collection and database

## Method 1: Running Magi Directly (Simple)

### 1. Download Magi

Download the Windows binary from the [releases page](https://github.com/alexander-bruun/magi/releases):

- **For 64-bit Windows**: `magi-windows-amd64.exe`
- **For ARM64 Windows**: `magi-windows-arm64.exe`

### 2. Create a Directory

Create a folder for Magi:

```powershell
mkdir C:\Magi
mv Downloads\magi-windows-amd64.exe C:\Magi\magi.exe
```

### 3. Run Magi

Open PowerShell or Command Prompt and run:

```powershell
cd C:\Magi
.\magi.exe
```

Magi will start and be accessible at `http://localhost:3000`.

> [!TIP]
> Keep the terminal window open while using Magi. Closing it will stop the server.

## Method 2: Windows Service with NSSM (Recommended)

Running Magi as a Windows Service allows it to start automatically and run in the background.

### 1. Download NSSM

[NSSM (Non-Sucking Service Manager)](https://nssm.cc/) is a free tool for creating Windows services.

1. Download NSSM from [nssm.cc/download](https://nssm.cc/download)
2. Extract to `C:\NSSM`

### 2. Download Magi

Download the Windows binary to `C:\Magi\magi.exe`.

### 3. Create Data Directory

```powershell
mkdir C:\MagiData
```

### 4. Install as Service

Open PowerShell **as Administrator**:

```powershell
cd C:\NSSM\win64

# Install Magi as a service
.\nssm.exe install Magi C:\Magi\magi.exe

# Set working directory
.\nssm.exe set Magi AppDirectory C:\MagiData

# Set environment variables
.\nssm.exe set Magi AppEnvironmentExtra MAGI_DATA_DIR=C:\MagiData

# Configure service to start automatically
.\nssm.exe set Magi Start SERVICE_AUTO_START

# Set service description
.\nssm.exe set Magi Description "Magi Manga Server"

# Configure service recovery on failure
.\nssm.exe set Magi AppRestartDelay 5000

# Start the service
.\nssm.exe start Magi
```

### 5. Verify Service

Check service status in Services management console:

1. Press `Win + R`
2. Type `services.msc`
3. Look for "Magi" in the list

Or use PowerShell:

```powershell
Get-Service Magi
```

## Method 3: Windows Service with Shawl

[Shawl](https://github.com/mtkennerly/shawl) is an alternative service wrapper.

### 1. Install Shawl

Download from [GitHub releases](https://github.com/mtkennerly/shawl/releases):

```powershell
# Using winget (Windows Package Manager)
winget install shawl

# Or using Scoop
scoop install shawl
```

### 2. Install Magi as Service

Open PowerShell **as Administrator**:

```powershell
shawl add `
  --name Magi `
  --cwd C:\MagiData `
  --env MAGI_DATA_DIR=C:\MagiData `
  --restart-if-crashed `
  -- C:\Magi\magi.exe

# Start the service
sc.exe start Magi
```

## Configuration

### Environment Variables

#### For Direct Execution

Set environment variables before running:

```powershell
$env:MAGI_DATA_DIR = "C:\MagiData"
$env:PORT = "3000"
.\magi.exe
```

#### For NSSM Service

```powershell
.\nssm.exe set Magi AppEnvironmentExtra MAGI_DATA_DIR=C:\MagiData PORT=3000
.\nssm.exe restart Magi
```

#### For Shawl Service

Edit the service:

```powershell
shawl edit Magi
```

### Custom Port

To use a different port:

```powershell
# NSSM
.\nssm.exe set Magi AppEnvironmentExtra PORT=8080
.\nssm.exe restart Magi

# Shawl (edit service configuration)
shawl edit Magi
```

### Manga Library Paths

Magi will scan directories you configure through the web interface. Common Windows paths:

- Local drive: `C:\Manga`
- Network share: `\\nas\Manga`
- External drive: `E:\Manga`

> [!IMPORTANT]
> When running as a service, ensure the SYSTEM account (or the user running the service) has read access to manga directories.

## Managing the Service

### NSSM Commands

```powershell
# Check service status
.\nssm.exe status Magi

# Start service
.\nssm.exe start Magi

# Stop service
.\nssm.exe stop Magi

# Restart service
.\nssm.exe restart Magi

# View service configuration
.\nssm.exe dump Magi

# Remove service
.\nssm.exe remove Magi confirm
```

### Windows Services Console

1. Press `Win + R`
2. Type `services.msc`
3. Find "Magi" in the list
4. Right-click for Start, Stop, Restart, Properties

### PowerShell Service Commands

```powershell
# Check status
Get-Service Magi

# Start service
Start-Service Magi

# Stop service
Stop-Service Magi

# Restart service
Restart-Service Magi

# Set to start automatically
Set-Service Magi -StartupType Automatic
```

## Viewing Logs

### NSSM Logs

Configure logging:

```powershell
# Enable stdout/stderr logging
.\nssm.exe set Magi AppStdout C:\MagiData\logs\magi.log
.\nssm.exe set Magi AppStderr C:\MagiData\logs\magi-error.log

# Rotate logs daily
.\nssm.exe set Magi AppRotateFiles 1
.\nssm.exe set Magi AppRotateOnline 1
.\nssm.exe set Magi AppRotateSeconds 86400

# Restart service to apply
.\nssm.exe restart Magi
```

View logs:

```powershell
# View latest logs
Get-Content C:\MagiData\logs\magi.log -Tail 50

# Follow logs in real-time
Get-Content C:\MagiData\logs\magi.log -Wait
```

### Windows Event Viewer

1. Press `Win + R`
2. Type `eventvwr.msc`
3. Navigate to: **Windows Logs > Application**
4. Filter by source "Magi" or "NSSM"

## Firewall Configuration

Allow Magi through Windows Firewall:

```powershell
# Add firewall rule (run as Administrator)
New-NetFirewallRule `
  -DisplayName "Magi Manga Server" `
  -Direction Inbound `
  -Protocol TCP `
  -LocalPort 3000 `
  -Action Allow `
  -Description "Allow inbound HTTP traffic to Magi"
```

Or use Windows Defender Firewall GUI:

1. Open **Windows Defender Firewall with Advanced Security**
2. Click **Inbound Rules**
3. Click **New Rule**
4. Select **Port**, click **Next**
5. Select **TCP**, enter port **3000**, click **Next**
6. Select **Allow the connection**, click **Next**
7. Apply to all profiles, click **Next**
8. Name it "Magi Manga Server", click **Finish**

## Updating Magi

### For Direct Execution

1. Download the new version
2. Stop Magi (close terminal)
3. Replace `C:\Magi\magi.exe`
4. Restart Magi

### For Service (NSSM)

```powershell
# Stop service
.\nssm.exe stop Magi

# Replace binary
mv Downloads\magi-windows-amd64.exe C:\Magi\magi.exe -Force

# Start service
.\nssm.exe start Magi
```

### For Service (Shawl)

```powershell
# Stop service
sc.exe stop Magi

# Replace binary
mv Downloads\magi-windows-amd64.exe C:\Magi\magi.exe -Force

# Start service
sc.exe start Magi
```

## Troubleshooting

### Service Won't Start

**Check Event Viewer:**
1. Open Event Viewer (`eventvwr.msc`)
2. Check Application logs for errors

**Common issues:**
- Port 3000 already in use → Change PORT environment variable
- Permission denied → Run service as user with proper permissions
- Missing dependencies → Ensure .NET Framework or VC++ Redistributable is installed

### Can't Access from Other Devices

1. Verify Windows Firewall rule is active
2. Check port forwarding on router (for external access)
3. Test locally first: `http://localhost:3000`
4. Test from same network: `http://[PC-IP]:3000`

### Permission Errors Reading Manga Files

If running as SYSTEM account:

```powershell
# Grant SYSTEM read access to manga folder
icacls "C:\Manga" /grant "SYSTEM:(OI)(CI)R" /T
```

Or configure service to run as your user:

```powershell
.\nssm.exe set Magi ObjectName ".\YourUsername" "YourPassword"
.\nssm.exe restart Magi
```

### High Memory Usage

Limit memory in NSSM (Windows 10/11 only):

```powershell
# Create a scheduled task with memory limit instead
```

## Accessing from Other Devices

On your network, access Magi from other devices using your PC's IP address:

```
http://192.168.1.100:3000
```

Find your IP:

```powershell
ipconfig | Select-String "IPv4"
```

## Uninstalling

### Remove NSSM Service

```powershell
# Stop service
.\nssm.exe stop Magi

# Remove service
.\nssm.exe remove Magi confirm
```

### Remove Files

```powershell
# Remove Magi
Remove-Item -Recurse -Force C:\Magi

# Remove data (optional - deletes database)
Remove-Item -Recurse -Force C:\MagiData

# Remove firewall rule
Remove-NetFirewallRule -DisplayName "Magi Manga Server"
```

## Next Steps

- [Create your first library](../usage/getting_started.md)
- [Configure automatic indexing](../usage/configuration.md)
- [Set up user accounts](../usage/configuration.md#user-management)
