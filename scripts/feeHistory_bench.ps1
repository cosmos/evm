param(
  [string]$Endpoint = "http://127.0.0.1:8545",
  [string]$Blocks = "0x40",
  [int]$Rounds = 8,
  [int[]]$Percentiles = @(25,50,75)
)

$body = @{ jsonrpc = "2.0"; id = 1; method = "eth_feeHistory"; params = @($Blocks, "latest", $Percentiles) } | ConvertTo-Json -Compress
$times = @()
Write-Host ("eth_feeHistory {0}, percentiles=[{1}], rounds={2}" -f $Blocks, ($Percentiles -join ","), $Rounds)
for ($i=1; $i -le $Rounds; $i++) {
  $sw = [System.Diagnostics.Stopwatch]::StartNew()
  try { Invoke-RestMethod -Uri $Endpoint -Method Post -ContentType "application/json" -Body $body | Out-Null } catch {}
  $sw.Stop()
  $ms = [int][Math]::Round($sw.Elapsed.TotalMilliseconds)
  Write-Host ("Run {0}: {1} ms" -f $i, $ms)
  $times += $ms
  Start-Sleep -Milliseconds 150
}
$avg = [Math]::Round(($times | Measure-Object -Average).Average,0)
$min = ($times | Measure-Object -Minimum).Minimum
$max = ($times | Measure-Object -Maximum).Maximum
Write-Host ("Avg: {0} ms   Min: {1} ms   Max: {2} ms" -f $avg, $min, $max)


