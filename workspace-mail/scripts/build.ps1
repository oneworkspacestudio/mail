param([string]$Go='go',[string]$Version='0.1.0-preview.1')
$ErrorActionPreference='Stop'
$root=Split-Path $PSScriptRoot -Parent
$expected='5f8370b259330e9eba65ed21bd59af92cfecd9597aea17594d803ec68b4bc291'
$url='https://github.com/stalwartlabs/stalwart/releases/download/v0.16.26/stalwart-x86_64-pc-windows-msvc.zip'
New-Item -ItemType Directory -Force "$root/payload","$root/dist" | Out-Null
$archive=Join-Path $root 'payload/stalwart.zip'
if(!(Test-Path -LiteralPath $archive)){Invoke-WebRequest -Uri $url -OutFile $archive}
if((Get-FileHash -LiteralPath $archive -Algorithm SHA256).Hash.ToLowerInvariant() -ne $expected){throw 'Stalwart archive checksum mismatch. Retain the file for inspection; no build was produced.'}
Push-Location $root
try{
 $env:CGO_ENABLED='0'
 & $Go build -buildvcs=false -trimpath -ldflags "-s -w -X main.version=$Version" -o dist/Mail.exe .
 if($LASTEXITCODE){throw 'Mail build failed'}
 $hash=(Get-FileHash dist/Mail.exe -Algorithm SHA256).Hash.ToLowerInvariant()
 [IO.File]::WriteAllText((Join-Path $root 'dist/SHA256SUMS.txt'),"$hash  Mail.exe`n",[Text.UTF8Encoding]::new($false))
 $manifest=@{schema=1;integration=1;version=$Version;asset='Mail.exe'}|ConvertTo-Json -Compress
 [IO.File]::WriteAllText((Join-Path $root 'dist/workspace-mail.json'),$manifest,[Text.UTF8Encoding]::new($false))
 $markerHash=(Get-FileHash dist/workspace-mail.json -Algorithm SHA256).Hash.ToLowerInvariant()
 [IO.File]::AppendAllText((Join-Path $root 'dist/SHA256SUMS.txt'),"$markerHash  workspace-mail.json`n",[Text.UTF8Encoding]::new($false))
 Copy-Item LICENSE dist/LICENSE.txt -Force
 Copy-Item NOTICE.md dist/NOTICE.md -Force
 Write-Output "Built $root/dist/Mail.exe"
}finally{Pop-Location}
