# Requires PowerShell 7+ (Windows, Linux or macOS).
# Example:
#   pwsh -File scripts/verify-bisync-integration.ps1 -Rclone ./rclone.exe
#   pwsh -File scripts/verify-bisync-integration.ps1 -Rclone ./rclone -WorkRoot ./test-output-new
# WorkRoot must not already exist. Files and logs remain there for inspection.
param(
    [string] $Rclone = 'rclone',
    [Alias('Evidence')]
    [string] $WorkRoot = (Join-Path ([System.IO.Path]::GetTempPath()) ('rclone-bisync-verify-' + [Guid]::NewGuid().ToString('N')))
)

$ErrorActionPreference = 'Stop'
if ($PSVersionTable.PSVersion.Major -lt 7) { throw 'This integration script requires PowerShell 7 or later.' }
$command = Get-Command -Name $Rclone -CommandType Application -ErrorAction SilentlyContinue
if ($null -eq $command) { throw "Rclone executable not found: $Rclone. Pass -Rclone <path> or add rclone to PATH." }
$Rclone = $command.Source
$Evidence = [System.IO.Path]::GetFullPath($WorkRoot)
if (Test-Path -LiteralPath $Evidence) {
    throw "WorkRoot already exists; choose a new empty path: $Evidence"
}
New-Item -ItemType Directory -Path $Evidence -ErrorAction Stop | Out-Null
$summary = Join-Path $Evidence 'results.json'
$results = [System.Collections.Generic.List[object]]::new()
$child = $null
[pscustomobject]@{
    executableSha256 = (Get-FileHash -Algorithm SHA256 -LiteralPath $Rclone).Hash.ToLowerInvariant()
    operatingSystem = [Runtime.InteropServices.RuntimeInformation]::OSDescription
    startedUtc = [DateTime]::UtcNow.ToString('o')
    remoteServiceContacted = $false
} | ConvertTo-Json | Set-Content -LiteralPath (Join-Path $Evidence 'environment.json') -Encoding utf8

function Assert([bool] $condition, [string] $message) {
    if (-not $condition) { throw $message }
}

function Case([string] $name) {
    $root = Join-Path $Evidence $name
    $left = Join-Path $root 'left'
    $right = Join-Path $root 'right'
    $state = Join-Path $root 'state'
    @($left, $right, $state) | ForEach-Object { New-Item -ItemType Directory -Path $_ -Force | Out-Null }
    return @{ Name = $name; Root = $root; Left = $left; Right = $right; State = $state; Config = (Join-Path $root 'missing-config.conf') }
}

function Run($c, [string] $label, [string[]] $extra = @()) {
    $args = @('bisync', $c.Left, $c.Right, '--config', $c.Config, '--workdir', $c.State, '-v') + $extra
    $log = Join-Path $c.Root ($label + '.log')
    $start = [Diagnostics.ProcessStartInfo]::new($Rclone)
    $start.UseShellExecute = $false
    $start.RedirectStandardOutput = $true
    $start.RedirectStandardError = $true
    foreach ($key in @($start.Environment.Keys)) {
        if ($key -like 'RCLONE_*' -or $key -eq '_RCLONE_CONFIG_KEY_FILE') { [void]$start.Environment.Remove($key) }
    }
    foreach ($arg in $args) { [void]$start.ArgumentList.Add($arg) }
    $process = [Diagnostics.Process]::Start($start)
    $stdout = $process.StandardOutput.ReadToEndAsync()
    $stderr = $process.StandardError.ReadToEndAsync()
    try {
        if (!$process.WaitForExit(120000)) {
            $process.Kill($true)
            [void]$process.WaitForExit(10000)
            throw "Disposable test $label exceeded its two-minute execution limit"
        }
        $code = $process.ExitCode
    } finally {
        [IO.File]::WriteAllText($log, $stdout.GetAwaiter().GetResult() + $stderr.GetAwaiter().GetResult())
        $process.Dispose()
    }
    $results.Add([pscustomobject]@{ Case = $c.Name; Step = $label; ExitCode = $code; Log = $log })
    return $code
}

function WriteText([string] $path, [string] $text) {
    [System.IO.File]::WriteAllText($path, $text)
}

function ReadText([string] $path) {
    return [System.IO.File]::ReadAllText($path)
}

try {
$preview = Case 'preview-and-init'
WriteText (Join-Path $preview.Left 'initial.txt') 'initial-content'
Assert ((Run $preview 'dry-preview' @('--resync-mode=path1', '--dry-run')) -eq 0) 'Preview failed'
Assert (-not (Test-Path (Join-Path $preview.Right 'initial.txt'))) 'Dry run wrote user data'
Assert ((Run $preview 'actual-init' @('--resync-mode=path1')) -eq 0) 'Initial bisync failed'
Assert ((ReadText (Join-Path $preview.Right 'initial.txt')) -eq 'initial-content') 'Initial file not copied'

WriteText (Join-Path $preview.Left 'left-added.txt') 'left-update'
WriteText (Join-Path $preview.Right 'right-added.txt') 'right-update'
Assert ((Run $preview 'two-way-edits') -eq 0) 'Two-way bisync failed'
Assert ((ReadText (Join-Path $preview.Right 'left-added.txt')) -eq 'left-update') 'Path1 change not transferred'
Assert ((ReadText (Join-Path $preview.Left 'right-added.txt')) -eq 'right-update') 'Path2 change not transferred'

$conflict = Case 'same-size-conflict'
WriteText (Join-Path $conflict.Left 'conflict.txt') 'ORIGINAL!'
WriteText (Join-Path $conflict.Left 'untouched.txt') 'unchanged-control'
Assert ((Run $conflict 'baseline' @('--resync-mode=path1')) -eq 0) 'Conflict baseline failed'
WriteText (Join-Path $conflict.Left 'conflict.txt') 'LEFTSIDE!'
WriteText (Join-Path $conflict.Right 'conflict.txt') 'RIGHTSIDE'
[System.IO.File]::SetLastWriteTimeUtc((Join-Path $conflict.Left 'conflict.txt'), [DateTime]::UtcNow.AddMinutes(-2))
[System.IO.File]::SetLastWriteTimeUtc((Join-Path $conflict.Right 'conflict.txt'), [DateTime]::UtcNow.AddMinutes(-1))
Assert ((Run $conflict 'two-sided-conflict' @('--conflict-resolve=none', '--conflict-loser=num')) -eq 0) 'Conflict resolution failed'
$allVersions = @((Get-ChildItem $conflict.Left, $conflict.Right -File -Recurse).FullName | ForEach-Object { ReadText $_ })
Assert ($allVersions -contains 'LEFTSIDE!') 'Left conflicting bytes were silently lost'
Assert ($allVersions -contains 'RIGHTSIDE') 'Right conflicting bytes were silently lost'
foreach ($side in @($conflict.Left, $conflict.Right)) {
    Assert ((ReadText (Join-Path $side 'conflict.txt.conflict1')) -eq 'LEFTSIDE!') 'Path1 conflict version missing after resolution'
    Assert ((ReadText (Join-Path $side 'conflict.txt.conflict2')) -eq 'RIGHTSIDE') 'Path2 conflict version missing after resolution'
}
$conflictLog = ReadText (Join-Path $conflict.Root 'two-sided-conflict.log')
Assert ($conflictLog -match '(?i)conflict') 'Both-sided changes did not exercise conflict handling'
Assert ($conflictLog -notmatch 'Safety abort: all files were changed') 'Conflict test was blocked by unrelated all-changed guard'

$deleteModify = Case 'delete-versus-modify'
1..20 | ForEach-Object { WriteText (Join-Path $deleteModify.Left ("control-$_.txt")) "control-$_" }
WriteText (Join-Path $deleteModify.Left 'deleted-left.txt') 'before-left'
WriteText (Join-Path $deleteModify.Left 'deleted-right.txt') 'before-right'
$integrity = @('--compare=size,checksum', '--checksum', '--require-checksum', '--max-delete=10')
Assert ((Run $deleteModify 'baseline' ($integrity + @('--resync-mode=path1'))) -eq 0) 'Delete/modify baseline failed'
Remove-Item -LiteralPath (Join-Path $deleteModify.Left 'deleted-left.txt')
WriteText (Join-Path $deleteModify.Right 'deleted-left.txt') 'modified-on-right-must-survive'
Remove-Item -LiteralPath (Join-Path $deleteModify.Right 'deleted-right.txt')
WriteText (Join-Path $deleteModify.Left 'deleted-right.txt') 'modified-on-left-must-survive'
Assert ((Run $deleteModify 'opposing-delete-and-modify' $integrity) -eq 0) 'Delete/modify reconciliation failed'
foreach ($side in @($deleteModify.Left, $deleteModify.Right)) {
    Assert ((ReadText (Join-Path $side 'deleted-left.txt')) -eq 'modified-on-right-must-survive') 'Path1 deletion erased the modified Path2 version'
    Assert ((ReadText (Join-Path $side 'deleted-right.txt')) -eq 'modified-on-left-must-survive') 'Path2 deletion erased the modified Path1 version'
}

$threshold = Case 'deletion-threshold'
1..10 | ForEach-Object { WriteText (Join-Path $threshold.Left ("file-$_.txt")) "keep-$_" }
Assert ((Run $threshold 'baseline' @('--resync-mode=path1')) -eq 0) 'Deletion baseline failed'
1..8 | ForEach-Object { Remove-Item -LiteralPath (Join-Path $threshold.Left ("file-$_.txt")) }
Assert ((Run $threshold 'blocked-delete' @('--max-delete=20')) -ne 0) 'Excessive deletion was accepted'
Assert ((ReadText (Join-Path $threshold.Root 'blocked-delete.log')) -match 'too many deletes') 'Deletion was blocked for an unrelated reason'
Assert ((Get-ChildItem $threshold.Right -Filter 'file-*.txt' -File).Count -eq 10) 'Delete protection failed to preserve target'

$missing = Case 'missing-root'
WriteText (Join-Path $missing.Left 'keep.txt') 'keep'
Assert ((Run $missing 'baseline' @('--resync-mode=path1')) -eq 0) 'Missing-root baseline failed'
Rename-Item -LiteralPath $missing.Left -NewName 'left-offline'
Assert ((Run $missing 'missing-left') -ne 0) 'Missing folder was treated as an empty source'
Assert ((ReadText (Join-Path $missing.Root 'missing-left.log')) -match 'directory not found') 'Missing source was rejected for an unrelated reason'
Assert ((ReadText (Join-Path $missing.Right 'keep.txt')) -eq 'keep') 'Missing folder deleted target content'

$sentinel = Case 'missing-sentinel'
WriteText (Join-Path $sentinel.Left 'keep.txt') 'keep'
Assert ((Run $sentinel 'absent-checkfiles' @('--resync-mode=path1', '--check-access', '--check-filename=.bisync-probe')) -ne 0) 'Missing sentinels not rejected'
Assert ((ReadText (Join-Path $sentinel.Root 'absent-checkfiles.log')) -match 'Access test failed') 'Missing sentinel was rejected for an unrelated reason'
WriteText (Join-Path $sentinel.Left '.bisync-probe') ''
WriteText (Join-Path $sentinel.Right '.bisync-probe') ''
# These markers represent the same pre-existing availability sentinel. Give
# both an identical whole-second timestamp so this test isolates check-access
# from the local backend's nanosecond modtime-reconciliation behaviour.
$sentinelTime = [DateTime]::SpecifyKind([DateTime]::new(2024, 1, 2, 3, 4, 5), [DateTimeKind]::Utc)
foreach ($side in @($sentinel.Left, $sentinel.Right)) {
    [IO.File]::SetLastWriteTimeUtc((Join-Path $side '.bisync-probe'), $sentinelTime)
}
Assert ((Run $sentinel 'valid-checkfiles' @('--resync-mode=path1', '--check-access', '--check-filename=.bisync-probe')) -eq 0) 'Valid checkfiles rejected'
Remove-Item -LiteralPath (Join-Path $sentinel.Right '.bisync-probe') -Force
Assert ((Run $sentinel 'lost-checkfile' @('--check-access', '--check-filename=.bisync-probe')) -ne 0) 'Disappearing sentinel not rejected'
Assert ((ReadText (Join-Path $sentinel.Root 'lost-checkfile.log')) -match 'Access test failed') 'Lost sentinel was rejected for an unrelated reason'
Assert ((ReadText (Join-Path $sentinel.Right 'keep.txt')) -eq 'keep') 'Disappearing sentinel changed content'

$cancel = Case 'forced-interruption'
WriteText (Join-Path $cancel.Left 'seed.txt') 'seed'
Assert ((Run $cancel 'baseline' @('--resync-mode=path1')) -eq 0) 'Cancellation baseline failed'
$bigFile = Join-Path $cancel.Left 'big.bin'
$stream = [System.IO.File]::Open($bigFile, [System.IO.FileMode]::CreateNew)
try { $stream.SetLength(67108864) } finally { $stream.Dispose() }
$psi = [System.Diagnostics.ProcessStartInfo]::new($Rclone)
foreach ($arg in @('bisync', $cancel.Left, $cancel.Right, '--config', $cancel.Config, '--workdir', $cancel.State, '--bwlimit=64k', '--transfers=1', '-q')) {
    [void]$psi.ArgumentList.Add($arg)
}
$psi.UseShellExecute = $false
foreach ($key in @($psi.Environment.Keys)) {
    if ($key -like 'RCLONE_*' -or $key -eq '_RCLONE_CONFIG_KEY_FILE') { [void]$psi.Environment.Remove($key) }
}
$child = [System.Diagnostics.Process]::Start($psi)
$foundLock = $false
for ($i = 0; $i -lt 150; $i++) {
    if (@(Get-ChildItem -LiteralPath $cancel.State -Filter '*.lck' -File).Count -gt 0) { $foundLock = $true; break }
    if ($child.HasExited) { break }
    Start-Sleep -Milliseconds 200
}
Assert $foundLock 'Could not observe a live bisync lock before forced cancellation'
Assert (-not $child.HasExited) 'Transfer completed before cancellation'
$reverseLog = Join-Path $cancel.Root 'reversed-concurrent.log'
$reverseArgs = @('bisync', $cancel.Right, $cancel.Left, '--config', $cancel.Config, '--workdir', $cancel.State, '--resync-mode=path1', '--dry-run', '-v')
& $Rclone @reverseArgs *> $reverseLog
$reverseCode = $LASTEXITCODE
$results.Add([pscustomobject]@{ Case = $cancel.Name; Step = 'reversed-concurrent'; ExitCode = $reverseCode; Log = $reverseLog })
Assert ($reverseCode -ne 0) 'Reverse order unexpectedly acquired concurrent pair lock'
Assert ((ReadText $reverseLog) -match 'another bisync run owns') 'Reverse order failed for a reason other than shared pair lock'
$child.Kill()
Assert ($child.WaitForExit(10000)) 'Killed child did not terminate'
$results.Add([pscustomobject]@{ Case = $cancel.Name; Step = 'forced-kill'; ExitCode = $child.ExitCode; Log = '' })
Assert ((Run $cancel 'interrupted-restart') -ne 0) 'Interrupted run resumed without acknowledging a stale baseline'
Assert ((ReadText (Join-Path $cancel.Root 'interrupted-restart.log')) -match 'prior lock file found') 'Interrupted restart failed for an unrelated reason'
}
finally {
    if ($null -ne $child) {
        try {
            if (-not $child.HasExited) {
                $child.Kill()
                [void] $child.WaitForExit(10000)
            }
        } finally {
            $child.Dispose()
        }
    }
    $results | ConvertTo-Json -Depth 5 | Set-Content -LiteralPath $summary -Encoding utf8
}

Write-Output "PASS: disposable CLI cases completed. Evidence: $summary"
exit 0
