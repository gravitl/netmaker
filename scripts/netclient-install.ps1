new-module -name netclient-install -scriptblock {
    function Quit {
        param(
            $Text
        )
        Write-Host "Exiting: " $Text
        Break Script
    }
    Function Netclient-Install() {
        param ($version='latest', $token)


    if($token -eq $null -or $token -eq ""){
        Quit "-token required"
    }

    $software = "WireGuard";
    $installed = (Get-ItemProperty HKLM:\Software\Microsoft\Windows\CurrentVersion\Uninstall\* | Where { $_.DisplayName -eq $software }) -ne $null

    If(-Not $installed) {
        Write-Host "'$software' is NOT installed. installing...";
        $url = "https://download.wireguard.com/windows-client/wireguard-installer.exe"
        $outpath = "$env:userprofile\Downloads\wireguard-installer.exe"
        Invoke-WebRequest -Uri $url -OutFile $outpath
        $args = @("Comma","Separated","Arguments")
        $procWG = Start-Process -Filepath "$env:userprofile\Downloads\wireguard-installer.exe" -ArgumentList $args
        if ($procWG -eq $null) {}
            Start-Sleep -Seconds 5
        } else {
            $procWG.WaitForExit() 
        }
        $procWG.WaitForExit()        
        Start-Sleep -Seconds 5
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
        $outpath = "$env:userprofile\Downloads\netclient.exe"
        Write-Host "'netclient.exe' is NOT installed. installing...";
        Write-Host "https://github.com/gravitl/netmaker/releases/download/$version/netclient.exe";
        $url = "https://github.com/gravitl/netmaker/releases/download/$version/netclient.exe"
        Invoke-WebRequest -Uri $url -OutFile $outpath
    }
    $NetArgs = @("join","-t",$token)
    $procNC = Start-Process -Filepath $outpath -ArgumentList $NetArgs
    if ($procNC -eq $null) {}
        Start-Sleep -Seconds 5
    } else {
        $procNC.WaitForExit() 
    }
    Add-MpPreference -ExclusionPath "C:\ProgramData\Netclient"

    if ((Get-Command "netclient.exe" -ErrorAction SilentlyContinue) -eq $null) { 
        if (-not (Test-Path -Path "C:\ProgramData\Netclient\bin\netclient.exe")) {
            New-Item -Path "C:\ProgramData\Netclient" -Name "bin" -ItemType "directory"
            Move-Item -Path "$env:userprofile\Downloads\netclient.exe" -Destination "C:\ProgramData\Netclient\bin\netclient.exe"
            $oldpath = (Get-ItemProperty -Path 'Registry::HKEY_LOCAL_MACHINE\System\CurrentControlSet\Control\Session Manager\Environment' -Name PATH).path
            $newpath = "$oldpath;C:\ProgramData\Netclient\bin"
            Set-ItemProperty -Path 'Registry::HKEY_LOCAL_MACHINE\System\CurrentControlSet\Control\Session Manager\Environment' -Name PATH -Value $newPath
            $env:Path += ";C:\ProgramData\Netclient\bin"
        }
        '''
        Please add netclient.exe to your path to make it permanently executable from powershell:
            1. Open "Edit environment variables for your account"
            2. Double click on "Path"
            3. On a new line, add the following: C:\ProgramData\Netclient\bin
            4. Click "Ok"
        '''
    }

    Write-Host "'netclient' is installed."
    }
}
