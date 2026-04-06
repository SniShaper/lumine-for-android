param(
    [string]$AndroidHome = "D:\Android\Sdk",
    [string]$JavaHome = "D:\Android\jbr",
    [string]$Output = "android\libs\LumineCore.aar",
    [int]$AndroidApi = 24,
    [string]$Package = "./mobile"
)

$ErrorActionPreference = "Stop"

function Ensure-Junction {
    param(
        [Parameter(Mandatory = $true)][string]$Path,
        [Parameter(Mandatory = $true)][string]$Target
    )

    if (Test-Path $Path) {
        return
    }
    if (!(Test-Path $Target)) {
        return
    }

    $parent = Split-Path -Parent $Path
    if ($parent -and !(Test-Path $parent)) {
        New-Item -ItemType Directory -Path $parent | Out-Null
    }
    New-Item -ItemType Junction -Path $Path -Target $Target | Out-Null
}

function Backup-File {
    param([string]$Path)
    if (!(Test-Path $Path)) {
        return $null
    }
    $tmp = [System.IO.Path]::GetTempFileName()
    Copy-Item $Path $tmp -Force
    return $tmp
}

function Restore-File {
    param(
        [string]$Backup,
        [string]$Destination
    )
    if ($Backup -and (Test-Path $Backup)) {
        Copy-Item $Backup $Destination -Force
        Remove-Item $Backup -Force
    }
}

Ensure-Junction -Path (Join-Path $AndroidHome "ndk") -Target "D:\sdk\ndk"
Ensure-Junction -Path (Join-Path $AndroidHome "platforms") -Target "D:\sdk\platforms"
Ensure-Junction -Path (Join-Path $AndroidHome "platform-tools") -Target "D:\sdk\platform-tools"
Ensure-Junction -Path (Join-Path $AndroidHome "build-tools") -Target "D:\sdk\build-tools"
Ensure-Junction -Path "D:\sdk\platforms\android-36" -Target "D:\sdk\platforms\android-36.1"

$goModBackup = Backup-File "go.mod"
$goSumBackup = Backup-File "go.sum"

try {
    $env:ANDROID_HOME = $AndroidHome
    $env:ANDROID_SDK_ROOT = $AndroidHome
    $env:JAVA_HOME = $JavaHome
    $env:GOFLAGS = "-mod=mod"
    $env:Path = "$JavaHome\bin;$AndroidHome\platform-tools;$env:Path"

    $outDir = Split-Path -Parent $Output
    if ($outDir -and !(Test-Path $outDir)) {
        New-Item -ItemType Directory -Path $outDir | Out-Null
    }

    $gomobileArgs = @(
        "bind"
        "-target=android"
        "-androidapi"
        "$AndroidApi"
        "-o"
        $Output
        $Package
    )
    & gomobile @gomobileArgs
    if ($LASTEXITCODE -ne 0) {
        exit $LASTEXITCODE
    }
}
finally {
    Restore-File -Backup $goModBackup -Destination "go.mod"
    Restore-File -Backup $goSumBackup -Destination "go.sum"
}
