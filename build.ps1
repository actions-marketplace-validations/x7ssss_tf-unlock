[CmdletBinding()]
param(
    [switch]$Test,
    [switch]$Dist,
    [switch]$Clean
)

$ErrorActionPreference = "Stop"

$BinaryName = "tf-unlock"
$Pkg = "."
$LdFlags = "-s -w"

if ($Clean) {
    Write-Host "Cleaning build artifacts..." -ForegroundColor Cyan
    if (Test-Path "dist") { Remove-Item -Recurse -Force "dist" }
    if (Test-Path "$BinaryName.exe") { Remove-Item -Force "$BinaryName.exe" }
    if (Test-Path "$BinaryName") { Remove-Item -Force "$BinaryName" }
    Write-Host "Clean complete." -ForegroundColor Green
    return
}

if ($Test) {
    Write-Host "Running tests..." -ForegroundColor Cyan
    go test -v ./...
    if ($LASTEXITCODE -ne 0) {
        Write-Error "Tests failed."
        exit $LASTEXITCODE
    }
    Write-Host "All tests passed." -ForegroundColor Green
    if (-not $Dist) { return }
}

if ($Dist) {
    Write-Host "Cross-compiling binaries into dist/..." -ForegroundColor Cyan
    if (-not (Test-Path "dist")) { New-Item -ItemType Directory -Path "dist" | Out-Null }

    $Targets = @(
        @{ OS = "windows"; Arch = "amd64"; Output = "dist/tf-unlock-windows-amd64.exe" },
        @{ OS = "windows"; Arch = "arm64"; Output = "dist/tf-unlock-windows-arm64.exe" },
        @{ OS = "linux";   Arch = "amd64"; Output = "dist/tf-unlock-linux-amd64" },
        @{ OS = "linux";   Arch = "arm64"; Output = "dist/tf-unlock-linux-arm64" },
        @{ OS = "darwin";  Arch = "amd64"; Output = "dist/tf-unlock-darwin-amd64" },
        @{ OS = "darwin";  Arch = "arm64"; Output = "dist/tf-unlock-darwin-arm64" }
    )

    foreach ($t in $Targets) {
        Write-Host "Building $($t.OS)/$($t.Arch) -> $($t.Output)..."
        $env:GOOS = $t.OS
        $env:GOARCH = $t.Arch
        $env:CGO_ENABLED = "0"
        go build -ldflags "$LdFlags" -o $t.Output $Pkg
        if ($LASTEXITCODE -ne 0) {
            Write-Error "Failed to build $($t.OS)/$($t.Arch)"
            exit $LASTEXITCODE
        }
    }

    # Reset environment
    $env:GOOS = $null
    $env:GOARCH = $null
    $env:CGO_ENABLED = $null

    Write-Host "Cross-compilation complete. Artifacts in dist/:" -ForegroundColor Green
    Get-ChildItem "dist" | Select-Object Name, Length, @{Name="MB"; Expression={[math]::Round($_.Length / 1MB, 2)}}
    return
}

# Default build
Write-Host "Building $BinaryName for host..." -ForegroundColor Cyan
go build -ldflags "$LdFlags" -o "$BinaryName.exe" $Pkg
if ($LASTEXITCODE -ne 0) {
    Write-Error "Build failed."
    exit $LASTEXITCODE
}
Write-Host "Build complete: $BinaryName.exe" -ForegroundColor Green
