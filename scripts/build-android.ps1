param(
    [ValidateSet("Debug", "Release")]
    [string]$Variant = "Debug",
    [string]$JavaHome = "",
    [string]$GomobileAndroidHome = "",
    [switch]$SkipBind
)

$ErrorActionPreference = "Stop"

$repoRoot = Split-Path -Parent $PSScriptRoot
if (-not $JavaHome) {
    $JavaHome = if ($env:JAVA_HOME) { $env:JAVA_HOME } else { "D:\Jetbrains IDES\Android Studio\jbr" }
}
if (-not $GomobileAndroidHome) {
    $GomobileAndroidHome = if ($env:ANDROID_HOME) { $env:ANDROID_HOME } elseif ($env:ANDROID_SDK_ROOT) { $env:ANDROID_SDK_ROOT } else { "D:\AndroidSDKs" }
}
$androidDir = Join-Path $repoRoot "android"
$bindScript = Join-Path $PSScriptRoot "gomobile-bind.ps1"
$gradleWrapper = Join-Path $androidDir "gradlew.bat"
$aarPath = Join-Path $androidDir "app\libs\LumineCore.aar"

if (!(Test-Path $gradleWrapper)) {
    throw "Gradle wrapper not found: $gradleWrapper"
}
if (!(Test-Path $JavaHome)) {
    throw "JavaHome not found: $JavaHome"
}

function Find-GoSourceNewestTime {
    param([string]$Root)
    $latest = [datetime]::MinValue
    foreach ($rel in @("enimul", "tun2socks")) {
        $base = Join-Path $Root $rel
        if (!(Test-Path $base)) { continue }
        foreach ($pattern in @("*.go", "*.mod", "*.sum")) {
            Get-ChildItem $base -Recurse -Filter $pattern -File -ErrorAction SilentlyContinue | ForEach-Object {
                if ($_.LastWriteTime -gt $latest) { $latest = $_.LastWriteTime }
            }
        }
    }
    return $latest
}

$needBind = $false
if ($SkipBind) {
    Write-Host "[build] -SkipBind given, AAR left unchanged"
}
elseif (!(Test-Path $aarPath)) {
    Write-Host "[build] LumineCore.aar not found, gomobile bind required"
    $needBind = $true
}
else {
    $srcTime = Find-GoSourceNewestTime $repoRoot
    $aarTime = (Get-Item $aarPath).LastWriteTime
    if ($srcTime -gt $aarTime.AddSeconds(2)) {
        Write-Host "[build] Go sources newer than AAR ($($srcTime.ToString('HH:mm:ss')) > $($aarTime.ToString('HH:mm:ss'))), rebuilding AAR"
        $needBind = $true
    }
    else {
        Write-Host "[build] AAR up to date ($($aarTime.ToString('yyyy-MM-dd HH:mm:ss'))), skipping gomobile bind"
    }
}

if ($needBind) {
    if (!(Test-Path $bindScript)) {
        throw "Bind script not found: $bindScript"
    }
    & $bindScript -AndroidHome $GomobileAndroidHome -JavaHome $JavaHome
    if ($LASTEXITCODE -ne 0) {
        exit $LASTEXITCODE
    }
}

$env:JAVA_HOME = $JavaHome
$env:Path = "$JavaHome\bin;$env:Path"

$task = if ($Variant -eq "Release") { "assembleRelease" } else { "assembleDebug" }
Push-Location $androidDir
try {
    & $gradleWrapper $task
    if ($LASTEXITCODE -ne 0) {
        exit $LASTEXITCODE
    }
}
finally {
    Pop-Location
}
