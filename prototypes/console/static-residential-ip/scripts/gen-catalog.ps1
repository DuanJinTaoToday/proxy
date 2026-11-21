# 从皇冠目录快照生成 src/catalog.ts。快照由只读查询脚本写到仓库根的 .scratch/ipcrown-catalog.json，不含凭证。
# 用法（仓库根目录）：pwsh prototypes/console/static-residential-ip/scripts/gen-catalog.ps1
param(
  [string]$Snapshot = '.scratch/ipcrown-catalog.json',
  [string]$Out = 'prototypes/console/static-residential-ip/src/catalog.ts'
)
$ErrorActionPreference = 'Stop'
$j = Get-Content $Snapshot -Raw | ConvertFrom-Json -Depth 50
$codes = @{ '美国'='US'; '韩国'='KR'; '菲律宾'='PH'; '荷兰'='NL'; '迪拜'='AE'; '巴西'='BR'; '意大利'='IT'; '西班牙'='ES'; '澳大利亚'='AU'; '中国香港'='HK'; '越南'='VN'; '日本'='JP'; '中国台湾'='TW'; '印度尼西亚'='ID'; '法国'='FR'; '德国'='DE'; '英国'='GB' }
$projectIds = 8, 9, 10

function Get-Product($cid, $projectId) { $j.products | Where-Object { $_.cid -eq $cid -and "$($_.pid)" -eq "$projectId" } | Select-Object -First 1 }
function Get-Stock($product, $id) { ($product.nodes | Where-Object id -eq $id).max }
function Format-Prices($product, $id) {
  (($product.nodes | Where-Object id -eq $id).prices | Where-Object { $_.price_type -ne 'test2h' } |
    ForEach-Object { "{ code: '$($_.price_type)', label: '$($_.label)', yuan: $($_.price) }" }) -join ', '
}

$exclusive = Get-Product 200001 ''
$shared = Get-Product 28 ''
$sharedByProject = @{}
foreach ($id in $projectIds) { $sharedByProject[$id] = Get-Product 28 $id }
$names = @{}
foreach ($p in $j.projects) { $names[[int]$p.id] = $p.title }

$nodes = foreach ($n in ($exclusive.nodes | Sort-Object { [int]$_.id })) {
  if (-not $codes.ContainsKey($n.country)) { throw "缺少国家代码：$($n.country)" }
  $province = if ($n.province -and -not $n.country.Contains($n.province)) { "'$($n.province)'" } else { 'undefined' }
  $projectStock = ($projectIds | ForEach-Object { "$($_): $(Get-Stock $sharedByProject[$_] $n.id)" }) -join ', '
  @"
  { nodeId: $($n.id), code: '$($codes[$n.country])', country: '$($n.country)', province: $province, continent: '$($n.continent)', isp: '$($n.isp)', line: '$($n.line)',
    exclusive: { stock: $(Get-Stock $exclusive $n.id), prices: [$(Format-Prices $exclusive $n.id)] },
    shared: { stock: { none: $(Get-Stock $shared $n.id), $projectStock }, prices: [$(Format-Prices $shared $n.id)] } },
"@
}

$queriedAt = ([datetime]$j.queriedAt).ToString('yyyy-MM-dd HH:mm')
$projects = ($projectIds | ForEach-Object { "  { pid: $_, label: '$($names[$_])' }," }) -join "`n"
$content = @"
// 由 scripts/gen-catalog.ps1 生成，勿手改。来源：皇冠 IPCrown 正式 API 目录快照（北京时间 $queriedAt）。
// 独享 cid=200001，共享 cid=28。yuan 为供应商目录报价（人民币元），已去掉 0 元的 test2h 测试卡。
// 共享库存随项目 pid 变化，快照只含不指定项目及 pid $($projectIds -join '/')。

export interface SupplierPrice { code: string, label: string, yuan: number }
export type ProjectKey = 'none' | $($projectIds -join ' | ')
export interface SupplierNode {
  nodeId: number
  code: string
  country: string
  province?: string
  continent: string
  isp: string
  line: string
  exclusive: { stock: number, prices: SupplierPrice[] }
  shared: { stock: Record<ProjectKey, number>, prices: SupplierPrice[] }
}

export const catalogQueriedAt = '$queriedAt'

export const supplierProjects: { pid: ProjectKey, label: string }[] = [
  { pid: 'none', label: '不指定用途' },
$projects
]

export const supplierNodes: SupplierNode[] = [
$($nodes -join "`n")
]
"@
Set-Content -Path $Out -Value $content -Encoding utf8NoBOM
Write-Output "已生成 $Out：$(@($nodes).Count) 个节点"
