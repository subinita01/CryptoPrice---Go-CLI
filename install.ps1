$ErrorActionPreference = 'Stop'

$repoRoot = Split-Path -Parent $MyInvocation.MyCommand.Path
$binaryName = 'cryptoprice.exe'
$destination = Join-Path $repoRoot $binaryName

Write-Host 'Building cryptoprice...'
go build -o $destination .

Write-Host "Built $destination"
Write-Host 'Add this folder to your PATH if you want to run cryptoprice from anywhere.'
