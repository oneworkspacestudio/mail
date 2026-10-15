param(
 [Parameter(Mandatory)][string]$SharedPackage,
 [string]$Go='go',
 [string]$Version='0.1.0-shared-preview.1'
)
$ErrorActionPreference='Stop'
$root=Split-Path $PSScriptRoot -Parent
$package=(Resolve-Path -LiteralPath $SharedPackage).Path
$checks=@{}
Get-Content -LiteralPath (Join-Path $package 'SHA256SUMS.txt') | ForEach-Object {if($_ -match '^([a-f0-9]{64})\s+(.+)$'){$checks[$matches[2]]=$matches[1]}}
$required=@('WorkspaceServices.exe','stalwart.exe','engine.lock.json')
foreach($name in $required){$path=Join-Path $package $name;if(-not $checks.ContainsKey($name) -or (Get-FileHash -LiteralPath $path -Algorithm SHA256).Hash.ToLowerInvariant() -ne $checks[$name]){throw "Shared package verification failed: $name"}}
$out=Join-Path $root 'dist/shared'
New-Item -ItemType Directory -Force $out | Out-Null
Push-Location $root
try{
 $env:CGO_ENABLED='0'
 & $Go build -buildvcs=false -trimpath -ldflags "-s -w -X main.version=$Version" -o (Join-Path $out 'Mail.exe') .
 if($LASTEXITCODE){throw 'Mail build failed'}
 foreach($name in $required){Copy-Item -LiteralPath (Join-Path $package $name) -Destination (Join-Path $out $name)}
 Copy-Item -LiteralPath (Join-Path $root 'LICENSE') -Destination (Join-Path $out 'LICENSE.txt')
 Copy-Item -LiteralPath (Join-Path $root 'NOTICE.md') -Destination (Join-Path $out 'NOTICE.md')
 Get-ChildItem -LiteralPath $out -File | Where-Object {$_.Name -ne 'SHA256SUMS.txt'} | ForEach-Object {((Get-FileHash -LiteralPath $_.FullName -Algorithm SHA256).Hash.ToLowerInvariant())+'  '+$_.Name} | Set-Content (Join-Path $out 'SHA256SUMS.txt')
 Write-Output "Built shared-mode Mail in $out. No backend was installed or started."
}finally{Pop-Location}
