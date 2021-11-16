param ($version='latest', $token)

function Quit {
    param(
        $Text
    )
    Write-Host "Exiting: " $Text
    Break Script
}

if($token -eq $null -or $token -eq ""){
    Quit "-token required"
}

$software = "WireGuard";
$installed = (Get-ItemProperty HKLM:\Software\Microsoft\Windows\CurrentVersion\Uninstall\* | Where { $_.DisplayName -eq $software }) -ne $null

If(-Not $installed) {
    Write-Host "'$software' is NOT installed. installing...";
    $url = "https://download.wireguard.com/windows-client/wireguard-installer.exe"
    $outpath = "$PSScriptRoot/wireguard-installer.exe"
    Invoke-WebRequest -Uri $url -OutFile $outpath
    $args = @("Comma","Separated","Arguments")
    Start-Process -Filepath "$PSScriptRoot/wireguard-installer.exe" -ArgumentList $args
    $software = "WireGuard";
    $installed = (Get-ItemProperty HKLM:\Software\Microsoft\Windows\CurrentVersion\Uninstall\* | Where { $_.DisplayName -eq $software }) -ne $null
    If(-Not $installed) {
        Quit "Could not install WireGuard"
    } else {
        Write-Host "'$software' is installed."
    }
} else {
	Write-Host "'$software' is installed."
}
$outpath = "";
if (Test-Path -Path "C:\ProgramData\Netclient\bin\netclient.exe") {
    $outpath = "C:\ProgramData\Netclient\bin\netclient.exe";
} else {
    $outpath = "$PSScriptRoot/netclient.exe"
    Write-Host "'netclient.exe' is NOT installed. installing...";
    Write-Host "https://github.com/gravitl/netmaker/releases/download/$version/netclient.exe";
    $url = "https://github.com/gravitl/netmaker/releases/download/$version/netclient.exe"
    Invoke-WebRequest -Uri $url -OutFile $outpath
}
$NetArgs = @("join","-t",$token)
Start-Process -Filepath $outpath -ArgumentList $NetArgs

if ((Get-Command "netclient.exe" -ErrorAction SilentlyContinue) -eq $null) { 
    if (-not (Test-Path -Path "C:\ProgramData\Netclient\bin\netclient.exe")) {
        New-Item -Path "C:\ProgramData\Netclient" -Name "bin" -ItemType "directory"
        Move-Item -Path "$PSScriptRoot/netclient.exe" -Destination "C:\ProgramData\Netclient\bin\netclient.exe"
    }
    '''
    Please add netclient.exe to your path to make it executable from powershell:
        1. Open "Edit environment variables for your account"
        2. Double click on "Path"
        3. On a new line, paste the following: %USERPROFILE%\AppData\Netclient\bin
        4. Click "Ok"
    '''
}

Write-Host "'netclient' is installed."
