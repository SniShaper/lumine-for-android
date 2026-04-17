param(
    [ValidateSet("Debug", "Release")]
    [string]$Variant = "Debug",
    [string]$JavaHome = "D:\Android\jbr",
    [string]$GomobileAndroidHome = "D:\Android\Sdk",
    [switch]$SkipBind
)

$ErrorActionPreference = "Stop"

$repoRoot = Split-Path -Parent $PSScriptRoot
$androidDir = Join-Path $repoRoot "android"
$bindScript = Join-Path $PSScriptRoot "gomobile-bind.ps1"
$gradleWrapper = Join-Path $androidDir "gradlew.bat"

if (!(Test-Path $gradleWrapper)) {
    throw "Gradle wrapper not found: $gradleWrapper"
}
if (!(Test-Path $JavaHome)) {
    throw "JavaHome not found: $JavaHome"
}

if (!$SkipBind) {
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
