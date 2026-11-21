package workspace

const windowsInstaller = `param(
 [Parameter(Mandatory=$true)][string]$BaseUrl,
 [Parameter(Mandatory=$true)][ValidatePattern('^[a-zA-Z0-9_-]+$')][string]$Tenant,
 [Parameter(Mandatory=$true)][ValidateSet('ap-south-1','eu-central-1')][string]$Region,
 [Parameter(Mandatory=$true)][ValidatePattern('^host_[0-9a-f]{8}$')][string]$CollectorId,
 [Parameter(Mandatory=$true)][string]$Token,
 [string]$ProbeTarget='', [string]$Window='2s', [string]$CaCert=''
)
$ErrorActionPreference='Stop'
$base=[Uri]$BaseUrl
if($base.Scheme -ne 'https' -or $base.UserInfo -or $base.Query -or $base.Fragment -or $base.AbsolutePath -ne '/') { throw 'An HTTPS receiver origin is required' }
if(-not [Environment]::Is64BitOperatingSystem){throw 'Windows x64 is required'}
[Net.ServicePointManager]::SecurityProtocol=[Net.SecurityProtocolType]::Tls12
$state=Join-Path $env:LOCALAPPDATA 'Kalki'
$bin=Join-Path $state ('bin\'+$CollectorId)
$data=Join-Path $state 'data'
New-Item -ItemType Directory -Force -Path $bin,$data | Out-Null
function Download-Kalki([string]$Url,[string]$File) {
 $part=$File+'.download'
 if($CaCert){ & curl.exe --fail --silent --show-error --ssl-revoke-best-effort --cacert $CaCert --proto '=https' $Url -o $part; if($LASTEXITCODE -ne 0){throw 'Download failed'} }
 else { Invoke-WebRequest -UseBasicParsing -Uri $Url -OutFile $part }
 Move-Item -LiteralPath $part -Destination $File -Force
}
$exe=Join-Path $bin 'collector-windows-amd64.exe'
$policy=Join-Path $bin 'allowlist.yaml'
Download-Kalki ($BaseUrl.TrimEnd('/')+'/downloads/collector-windows-amd64.exe') $exe
Download-Kalki ($BaseUrl.TrimEnd('/')+'/downloads/allowlist.yaml') $policy
if(-not $ProbeTarget){$ProbeTarget=$base.DnsSafeHost+':'+$base.Port}
$previousToken=$env:KALKI_TOKEN
try {
 $env:KALKI_TOKEN=$Token
 $arguments=@('--mode=live',"--tenant=$Tenant","--region=$Region","--collector-id=$CollectorId","--tier2-url=$BaseUrl","--probe-target=$ProbeTarget","--window=$Window","--key-file=$(Join-Path $data ($Tenant+'.key'))","--wal-path=$(Join-Path $data ($CollectorId+'.wal'))","--allowlist=$policy","--ca-cert=$CaCert",'--proof-addr=127.0.0.1:0')
 & $exe @arguments
 if($LASTEXITCODE -ne 0){throw 'Collector stopped; see the local diagnostic above'}
} finally { $env:KALKI_TOKEN=$previousToken }
`
