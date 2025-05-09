#Requires -Version 5
#Requires -RunAsAdministrator
#Requires -Modules ScheduledTasks
<#
.SYNOPSIS
  Google Cloud Example Agent uninstall script.
.DESCRIPTION
  This powershell script is used to uninstall the Google Cloud Example Agent
  on the system and remove a Task Scheduler entry: google-cloud-example-agent-monitor,
  .
#>
$ErrorActionPreference = 'Stop'
if ($env:ProgramData -eq $null -or $env:ProgramData -eq '') {
  $DATA_DIR = 'C:\Program Files\Google\google-cloud-example-agent'
}
else {
  $DATA_DIR = $env:ProgramData + '\Google\google-cloud-example-agent'
}
$INSTALL_DIR = 'C:\Program Files\Google\google-cloud-example-agent'
$SVC_NAME = 'google-cloud-example-agent'
$MONITOR_TASK = 'google-cloud-example-agent-monitor'

function Log-Uninstall {
  #.DESCRIPTION
  #  Invokes the service with usage logging enabled to log an uninstall event
  try {
    Start-Process $INSTALL_DIR\$BIN_NAME_EXE -ArgumentList 'logusage','-s','UNINSTALLED' | Wait-Process -Timeout 30
  } catch {}
}

Log-Uninstall
try {
  # stop the service / tasks and remove them
  if ($(Get-ScheduledTask $MONITOR_TASK -ErrorAction Ignore).TaskName) {
    Disable-ScheduledTask $MONITOR_TASK
    Unregister-ScheduledTask -TaskName $MONITOR_TASK -Confirm:$false
  }
  if ($(Get-Service -Name $SVC_NAME -ErrorAction SilentlyContinue).Length -gt 0) {
    Stop-Service $SVC_NAME
    $service = Get-CimInstance -ClassName Win32_Service -Filter "Name='google-cloud-example-agent'"
    $service.Dispose()
    # without the ampersand PowerShell will block removal of the service for some time.
    & sc.exe delete $SVC_NAME
  }

  # remove the agent directory
  if (Test-Path $INSTALL_DIR) {
    Remove-Item -Recurse -Force $INSTALL_DIR
  }
  if (Test-Path $DATA_DIR) {
    Remove-Item -Recurse -Force $DATA_DIR
  }
}
catch {
  break
}
