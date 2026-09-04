param(
    [string]$AndroidHome = "",
    [string]$JavaHome = "",
    [string]$Output = "android\libs\LumineCore.aar",
    [int]$AndroidApi = 24,
    [string]$ModuleDir = "enimul",
    [string]$Package = "./mobile",
    [ValidateSet("all", "arm", "arm64", "386", "amd64")]
    [string]$Arch = "all"
)

$ErrorActionPreference = "Stop"

if (-not $AndroidHome) {
    $AndroidHome = if ($env:ANDROID_HOME) { $env:ANDROID_HOME } elseif ($env:ANDROID_SDK_ROOT) { $env:ANDROID_SDK_ROOT } else { "D:\AndroidSDKs" }
}
if (-not $JavaHome) {
    $JavaHome = if ($env:JAVA_HOME) { $env:JAVA_HOME } else { "D:\Jetbrains IDES\Android Studio\jbr" }
}

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

Ensure-Junction -Path (Join-Path $AndroidHome "ndk") -Target "D:\sdk\ndk"
Ensure-Junction -Path (Join-Path $AndroidHome "platforms") -Target "D:\sdk\platforms"
Ensure-Junction -Path (Join-Path $AndroidHome "platform-tools") -Target "D:\sdk\platform-tools"
Ensure-Junction -Path (Join-Path $AndroidHome "build-tools") -Target "D:\sdk\build-tools"
Ensure-Junction -Path "D:\sdk\platforms\android-36" -Target "D:\sdk\platforms\android-36.1"

$Output = [System.IO.Path]::GetFullPath((Join-Path (Get-Location) $Output))

$target = if ($Arch -eq "all") { "android" } else { "android/$Arch" }

Push-Location $ModuleDir

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
        "-target=$target"
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
    Pop-Location
}
