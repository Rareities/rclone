param(
    [Parameter(Mandatory)][string] $Tag,
    [Parameter(Mandatory)][string] $OutputDirectory,
    [ValidateSet('windows-amd64','linux-amd64','linux-arm64','darwin-amd64','darwin-arm64')]
    [string[]] $Targets = @('windows-amd64','linux-amd64','linux-arm64','darwin-amd64','darwin-arm64')
)
$ErrorActionPreference = 'Stop'
if($PSVersionTable.PSVersion.Major -lt 7){throw 'PowerShell 7 or later is required'}
$root = Split-Path $PSScriptRoot -Parent
if($Tag -notmatch '^v[0-9]+\.[0-9]+\.[0-9]+-cloudbridge\.[0-9]+$'){throw 'Invalid fork release tag'}
$sourceVersion = (Get-Content -LiteralPath (Join-Path $root 'VERSION') -Raw).Trim()
if(!$Tag.StartsWith($sourceVersion + '-cloudbridge.')){throw 'Fork version must retain the actual development source version'}
$notes = Join-Path $root "docs/releases/$Tag.md"
if(!(Test-Path -LiteralPath $notes)){throw 'The full release changelog is missing'}
$sha = (& git -C $root rev-parse HEAD).Trim()
if($LASTEXITCODE -ne 0 -or $sha -notmatch '^[0-9a-f]{40}$'){throw 'Cannot resolve source identity'}
if((& git -C $root status --porcelain)){throw 'Commit and verify all changes before producing release artifacts'}
$go = (& go version).Trim()
if($go -notmatch 'go([0-9]+)\.([0-9]+)\.([0-9]+)'){throw 'Cannot identify Go toolchain'}
$toolchain = [Version]::new([int]$Matches[1],[int]$Matches[2],[int]$Matches[3])
if($toolchain -lt [Version]'1.27.2'){throw 'Go 1.27.2 or newer is required for this release'}
if(Test-Path -LiteralPath $OutputDirectory){throw 'Choose a new output directory to exclude stale artifacts'}
$OutputDirectory = [IO.Path]::GetFullPath($OutputDirectory)
[void][IO.Directory]::CreateDirectory($OutputDirectory)
$old = @{}
foreach($name in @('GOOS','GOARCH','CGO_ENABLED')){$old[$name]=[Environment]::GetEnvironmentVariable($name,'Process')}
$results = [Collections.Generic.List[object]]::new()
Push-Location $root
try {
    $modules = & go list -m -mod=readonly -f '{{.Path}} {{.Version}}{{if .Replace}} => {{.Replace.Path}} {{.Replace.Version}}{{end}}' all
    if($LASTEXITCODE -ne 0){throw 'Module provenance failed'}
    foreach($target in $Targets){
        $parts = $target.Split('-')
        $env:GOOS=$parts[0]; $env:GOARCH=$parts[1]; $env:CGO_ENABLED='0'
        $stage = Join-Path $OutputDirectory "staging/$target/rclone-$Tag-$target"
        [void][IO.Directory]::CreateDirectory($stage)
        $name = if($env:GOOS -eq 'windows'){'rclone.exe'}else{'rclone'}
        $binary = Join-Path $stage $name
        & go build -mod=readonly -buildvcs=true -trimpath -tags noselfupdate -ldflags "-s -w -X github.com/rclone/rclone/fs.Version=$Tag" -o $binary .
        if($LASTEXITCODE -ne 0){throw "Native build failed: $target"}
        $metadata = @(& go version -m $binary)
        if($LASTEXITCODE -ne 0 -or !($metadata -match "vcs.revision=$sha") -or !($metadata -match 'vcs.modified=false')){
            throw "Binary source identity is not the committed release: $target"
        }
        $metadata | Set-Content -LiteralPath (Join-Path $stage 'build-info.txt') -Encoding utf8
        $modules | Set-Content -LiteralPath (Join-Path $stage 'modules.txt') -Encoding utf8
        Copy-Item -LiteralPath (Join-Path $root 'COPYING') -Destination $stage
        Copy-Item -LiteralPath $notes -Destination (Join-Path $stage 'CHANGELOG.md')
        $provenance = [pscustomobject]@{
            schemaVersion=1;repository='https://github.com/Rareities/rclone';commit=$sha
            tag=$Tag;sourceVersion=$sourceVersion;goToolchain=$go;goos=$parts[0];goarch=$parts[1]
            cgoEnabled=$false;buildTags=@('noselfupdate');binary=$name
            binarySha256=(Get-FileHash -LiteralPath $binary -Algorithm SHA256).Hash.ToLowerInvariant()
        }
        $provenance | ConvertTo-Json -Depth 5 | Set-Content -LiteralPath (Join-Path $stage 'provenance.json') -Encoding utf8
        $archive = Join-Path $OutputDirectory "rclone-$Tag-$target.zip"
        [IO.Compression.ZipFile]::CreateFromDirectory((Split-Path $stage -Parent), $archive, [IO.Compression.CompressionLevel]::Optimal, $false)
        if($parts[0] -ne 'windows') {
            $zip = [IO.Compression.ZipFile]::Open($archive, [IO.Compression.ZipArchiveMode]::Update)
            try {
                $entry = $zip.GetEntry("rclone-$Tag-$target/$name")
                if(!$entry){throw 'The native executable is missing from the archive'}
                $entry.ExternalAttributes = [BitConverter]::ToInt32([BitConverter]::GetBytes([uint32]2179792896),0)
            } finally { $zip.Dispose() }
        }
        $results.Add([pscustomobject]@{name=[IO.Path]::GetFileName($archive);sha256=(Get-FileHash -LiteralPath $archive -Algorithm SHA256).Hash.ToLowerInvariant();bytes=(Get-Item -LiteralPath $archive).Length;commit=$sha;target=$target})
    }
    $results | ConvertTo-Json -Depth 5 | Set-Content -LiteralPath (Join-Path $OutputDirectory 'release-manifest.json') -Encoding utf8
    Copy-Item -LiteralPath $notes -Destination (Join-Path $OutputDirectory 'CHANGELOG.md')
    Get-ChildItem -LiteralPath $OutputDirectory -File | Sort-Object Name | ForEach-Object{
        "$((Get-FileHash -LiteralPath $_.FullName -Algorithm SHA256).Hash.ToLowerInvariant())  $($_.Name)"
    } | Set-Content -LiteralPath (Join-Path $OutputDirectory 'SHA256SUMS.txt') -Encoding utf8
    Write-Output "Built $($results.Count) traceable fork archives from $sha"
} finally {
    Pop-Location
    foreach($name in $old.Keys){[Environment]::SetEnvironmentVariable($name,$old[$name],'Process')}
}
