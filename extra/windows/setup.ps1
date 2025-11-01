# The setup script installs laitos supplements for windows, and a scheduled task that starts laitos automatically.
$ErrorActionPreference = 'Stop'

$laitosCmd = Read-Host -Prompt 'What is the absolute path to laitos.exe? E.g. %USERPROFILE%\laitos.exe'
$laitosArg = Read-Host -Prompt 'What parameters to use for launching laitos automatically? E.g. -disableconflicts -awsinteg -prominteg -gomaxprocs 2 -config config.json -daemons autounlock,dnsd,httpd,httpproxy,insecurehttpd,maintenance,passwdrpc,phonehome,plainsocket,simpleipsvcd,smtpd,snmpd,sockd,telegram'
$laitosWD = Read-Host -Prompt 'Which directory does laitos program data (JSON config, web pages, etc) reside?'
$laitosAction = New-ScheduledTaskAction -Execute $laitosCmd -Argument $laitosArg -WorkingDirectory $laitosWD
$laitosTrigger = New-ScheduledTaskTrigger -AtLogOn
$laitosSettings = New-ScheduledTaskSettingsSet -AllowStartIfOnBatteries -DontStopIfGoingOnBatteries -StartWhenAvailable -DontStopOnIdleEnd -RestartCount 10000 -RestartInterval (New-TimeSpan -Minutes 3)
$laitosUser = Read-Host -Prompt 'What administrator user will laitos run as? E.g. Administrator'
$laitosPrincipal = New-ScheduledTaskPrincipal -UserID $laitosUser -RunLevel Highest
$laitosTask = New-ScheduledTask -Action $laitosAction -Trigger $laitosTrigger -Settings $laitosSettings -Principal $laitosPrincipal
Register-ScheduledTask -Force -TaskName laitos -InputObject $laitosTask

Read-Host -Prompt 'laitos is now ready to start automatically when the user logs on, press enter to exit.'
