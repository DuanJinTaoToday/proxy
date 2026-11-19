import brandLogo from '@console/assets/kunxing-mark.png'

const icons = {
  grid: '<rect x="3" y="3" width="7" height="7" rx="1"/><rect x="14" y="3" width="7" height="7" rx="1"/><rect x="3" y="14" width="7" height="7" rx="1"/><rect x="14" y="14" width="7" height="7" rx="1"/>',
  activity: '<path d="M3 12h4l3-7 4 14 3-7h4"/>',
  server: '<rect x="3" y="4" width="18" height="6" rx="2"/><rect x="3" y="14" width="18" height="6" rx="2"/><path d="M7 7h.01M7 17h.01"/>',
  users: '<path d="M16 21v-2a4 4 0 0 0-4-4H6a4 4 0 0 0-4 4v2m20 0v-2a4 4 0 0 0-3-4m-3-11a4 4 0 0 1 0 8"/><circle cx="9" cy="8" r="4"/>',
  wallet: '<path d="M20 8V6a2 2 0 0 0-2-2H5a2 2 0 0 0-2 2v12a2 2 0 0 0 2 2h13a2 2 0 0 0 2-2v-2M3 8h17a1 1 0 0 1 1 1v6a1 1 0 0 1-1 1h-4a4 4 0 0 1 0-8m0 4h.01"/>',
  box: '<path d="m12 3 9 5-9 5-9-5 9-5Zm9 5v8l-9 5-9-5V8m9 5v8"/>',
  chart: '<path d="M3 3v18h18M8 15v-4m5 4V7m5 8v-6"/>',
  arrow: '<path d="M4 12h15m-6-6 6 6-6 6"/>',
  chevron: '<path d="m9 18 6-6-6-6"/>', left: '<path d="m15 18-6-6 6-6"/>',
  sidebar: '<rect x="3" y="4" width="18" height="16" rx="2"/><path d="M9 4v16"/>',
  book: '<path d="M4 5h5a3 3 0 0 1 3 3v13a4 4 0 0 0-4-3H4V5Zm16 0h-5a3 3 0 0 0-3 3v13a4 4 0 0 1 4-3h4V5Z"/>',
  plus: '<path d="M12 5v14M5 12h14"/>',
  clock: '<circle cx="12" cy="12" r="9"/><path d="M12 7v5l3 2"/>',
  check: '<path d="m5 12 4 4L19 6"/>',
  terminal: '<path d="m5 7 5 5-5 5m8 0h6"/>',
  copy: '<rect x="8" y="8" width="12" height="12" rx="2"/><path d="M16 8V6a2 2 0 0 0-2-2H6a2 2 0 0 0-2 2v8a2 2 0 0 0 2 2h2"/>',
  close: '<path d="m6 6 12 12M18 6 6 18"/>',
  alert: '<path d="M12 8v4m0 4h.01M10.3 4.9 3 18a1.5 1.5 0 0 0 1.3 2h15.4a1.5 1.5 0 0 0 1.3-2L13.7 4.9a2 2 0 0 0-3.4 0Z"/>',
}
const icon = (name) => `<svg viewBox="0 0 24 24" width="18" height="18" fill="none" stroke="currentColor" stroke-width="1.65" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">${icons[name]}</svg>`
const escape = (value) => String(value).replace(/[&<>"']/g, character => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' }[character]))
const variants = [
  { id: 'resources', name: '资源概览' },
  { id: 'setup', name: '接入向导' },
  { id: 'operations', name: '日常工作台' },
]
const navigation = [
  ['', [['overview', 'grid', '账户概览']]],
  ['动态住宅', [['traffic', 'box', '购买流量'], ['usage', 'chart', '日用量']]],
  ['静态住宅', [['static', 'box', '购买静态 IP'], ['ips', 'server', '我的 IP']]],
  ['账户管理', [['accounts', 'users', '代理子账号'], ['wallet', 'wallet', '钱包与订单']]],
]
const pages = Object.fromEntries(navigation.flatMap(([, items]) => items.map(([id, glyph, label]) => [id, { glyph, label }])))
const sampleAccounts = {
  new: [],
  active: [
    { name: 'web01', remark: '站点连通性测试', password: 'DemoWeb01', limit: '10', used: '2.84' },
    { name: 'qa01', remark: '测试环境', password: 'DemoQa01', limit: '', used: '1.16' },
  ],
}
const copiedScenarios = new Set()
const usage = [
  { day: '周一', value: 0.42 }, { day: '周二', value: 0.83 }, { day: '周三', value: 0.62 },
  { day: '周四', value: 1.18 }, { day: '周五', value: 0.78 }, { day: '周六', value: 1.46 }, { day: '周日', value: 1.25 },
]
const params = new URLSearchParams(location.search)
let variant = variants.find(({ id }) => id === params.get('variant')) ?? variants[0]
let scenario = params.get('scenario') === 'new' ? 'new' : 'active'
let page = Object.hasOwn(pages, location.hash.slice(1)) ? location.hash.slice(1) : 'overview'
let configuration
let dialogTrigger
const main = document.querySelector('#main')
const dialog = document.querySelector('#dialog')
const mobile = matchMedia('(max-width: 760px)')
const hasResources = () => scenario === 'active'
const accounts = () => sampleAccounts[scenario]
const primaryAccount = () => hasResources() ? 'xinghe' : 'demo2026'
const link = (target, label, glyph = 'arrow') => `<a class="text-link" href="#${target}" data-page="${target}">${label}${icon(glyph)}</a>`
const action = (target, label, glyph, primary = false) => `<button class="button ${primary ? 'primary' : ''}" ${target === 'create' ? 'data-create' : `data-page="${target}"`}>${icon(glyph)}${label}</button>`

function balances() {
  return [
    { title: '动态流量余额', number: hasResources() ? '38.64' : '0.00', unit: 'GB', glyph: 'activity', detail: hasResources() ? '长期有效 · 用完为止' : '购买流量后可建立连接' },
    { title: '可用静态 IP', number: hasResources() ? '4' : '0', unit: '个', glyph: 'server', detail: hasResources() ? '2 个将在 5 天内到期' : '购买后在「我的 IP」查看' },
    { title: '钱包可用余额', number: hasResources() ? '286.50' : '0.00', unit: 'CNY', glyph: 'wallet', detail: '用于购买流量和静态 IP' },
  ]
}

function balanceCards() {
  return `<div class="balance-grid">${balances().map(item => `<section class="panel balance">
    <div class="balance-header"><h2>${item.title}</h2><span class="tile-icon">${icon(item.glyph)}</span></div>
    <p class="metric">${item.number} <small>${item.unit}</small></p>
    <p class="balance-note">${item.detail}</p>
  </section>`).join('')}</div>`
}

function balanceList() {
  return `<section class="panel"><div class="section-header"><h2>资源与余额</h2></div>
    <div class="balance-list">${balances().map(item => `<div class="resource-row"><span class="tile-icon">${icon(item.glyph)}</span>
    <div><strong>${item.title}</strong><p>${item.detail}</p></div><div class="resource-value">${item.number} <small>${item.unit}</small></div></div>`).join('')}</div></section>`
}

function graph() {
  const total = usage.reduce((sum, { value }) => sum + value, 0).toFixed(2)
  return `<section class="panel usage"><div class="section-header"><div><h2>动态流量用量</h2><p>最近 7 天 · 租户全部子账号</p></div>${link('usage', '查看日用量')}</div>
    ${hasResources() ? `<div class="usage-total"><strong>${total}</strong> <span>GB 已用</span></div>
    <div class="graph" aria-label="最近七天用量，以 GB 计">
      <div class="axis" aria-hidden="true"><span>1.5</span><span>1.0</span><span>0.5</span><span>0</span></div>
      <div class="plot"><div class="plot-grid" aria-hidden="true"></div><div class="bars">${usage.map(({ day, value }, index) => `<div class="bar-column"><div class="bar ${index === 6 ? 'current' : ''}" style="--height:${value / 1.5 * 100}%" tabindex="0" aria-label="${day} ${value} GB" title="${day} ${value} GB"><span class="bar-tip">${value} GB</span></div><span class="day">${day}</span></div>`).join('')}</div></div>
    </div>` : `<div class="empty chart-empty">${icon('chart')}<strong>暂无用量记录</strong><p>开始使用动态代理后，这里会显示每日用量。</p></div>`}
  </section>`
}

function reminders() {
  return `<section class="panel notices"><div class="section-header"><div><h2>需要关注</h2><p>资源到期与余额提醒</p></div>${hasResources() ? '<span class="counter">1</span>' : ''}</div>
    ${hasResources() ? `<div class="notice"><span class="tile-icon amber">${icon('clock')}</span><div><h3>2 个静态 IP 即将到期</h3><p>请在到期前完成续费，避免连接中断。</p>${link('ips', '查看即将到期的 IP')}</div></div>` : `<div class="empty small-empty">${icon('check')}<strong>暂无待处理提醒</strong></div>`}
  </section>`
}

function firstConnection() {
  return `<section class="panel first-connection"><span class="tile-icon accent">${icon('terminal')}</span><div><h2>${accounts().length ? '你的接入配置已就绪' : '创建第一个代理子账号'}</h2><p>${accounts().length ? '使用子账号配置 SOCKS5，购买流量后即可建立连接。' : '为项目创建独立账号，获取 SOCKS5 地址和账密。'}</p></div>
    ${accounts().length ? `<button class="button primary" data-config="0">${icon('copy')}查看接入配置</button>` : action('create', '创建代理子账号', 'plus', true)}</section>`
}

function setupChecklist() {
  const steps = [
    { title: '创建代理子账号', description: '为使用代理的项目分配一组独立账密。', complete: accounts().length > 0, button: action('create', '创建代理子账号', 'plus', true) },
    { title: '准备动态流量', description: '为租户购买动态流量，启用 SOCKS5 接入。', complete: hasResources(), button: action('traffic', '购买流量', 'activity') },
    { title: '复制 SOCKS5 配置', description: '将地址、端口和账密填入你的客户端。', complete: copiedScenarios.has(scenario), button: accounts().length ? `<button class="button" data-config="0">${icon('copy')}查看接入配置</button>` : `<p class="step-hint">创建子账号后可查看</p>` },
  ]
  return `<section class="panel checklist"><div class="section-header"><div><h2>配置动态代理</h2><p>从创建子账号到获得可用的接入配置。</p></div><span class="counter">${steps.filter(step => step.complete).length} / 3</span></div>
    <ol>${steps.map((step, index) => `<li><span class="step-number ${step.complete ? 'complete' : ''}">${step.complete ? icon('check') : index + 1}</span><div><h3>${step.title}</h3><p>${step.description}</p>${step.complete ? '<span class="done-label">已完成</span>' : step.button}${index === 2 && step.complete ? ` <button class="text-link" data-config="0">再次查看</button>` : ''}</div></li>`).join('')}</ol></section>`
}

function accountTable() {
  return `<section class="panel"><div class="section-header"><div><h2>代理子账号</h2><p>已创建 ${accounts().length} / 10 个</p></div>${action('create', '创建代理子账号', 'plus', true)}</div>
    ${accounts().length ? `<div class="table-wrap"><table><thead><tr><th scope="col">子账号</th><th scope="col">状态</th><th scope="col">累计用量 / 上限</th><th scope="col">接入</th></tr></thead><tbody>${accounts().map((account, index) => `<tr><td><strong>${escape(account.name)}</strong>${account.remark ? `<small>${escape(account.remark)}</small>` : ''}</td><td><span class="status-pill"><i></i>已启用</span></td><td>${account.used} GB / ${account.limit ? `${escape(account.limit)} GB` : '不限'}</td><td><button class="text-link" data-config="${index}">${icon('copy')}接入配置</button></td></tr>`).join('')}</tbody></table></div>` : `<div class="empty">${icon('users')}<strong>还没有代理子账号</strong><p>创建后可查看 SOCKS5 接入配置。</p></div>`}</section>`
}

function overview() {
  const primary = variant.id === 'resources' && hasResources() ? action('create', '创建代理子账号', 'plus', true) : ''
  const quick = `${action('traffic', '购买流量', 'activity')}${action('static', '购买静态 IP', 'server')}${primary}`
  const content = {
    resources: `${!hasResources() ? firstConnection() : ''}${balanceCards()}<div class="split">${graph()}${reminders()}</div>`,
    setup: `<div class="split setup-layout">${setupChecklist()}<div class="stack">${balanceList()}${reminders()}</div></div>`,
    operations: `${!hasResources() ? firstConnection() : ''}${balanceList()}<div class="split">${accountTable()}${reminders()}</div>`,
  }
  return `<div class="page-heading"><div><h1>账户概览</h1><p>${hasResources() ? '查看动态流量、静态 IP 和钱包的可用余额。' : '管理代理资源，完成首次接入。'}</p></div><div class="actions">${quick}</div></div><div class="stack">${content[variant.id]}</div>`
}

function relatedPage() {
  const panels = {
    accounts: accountTable,
    usage: graph,
    ips: () => `<section class="panel"><div class="section-header"><h2>静态 IP 列表</h2>${action('static', '购买静态 IP', 'server')}</div>${hasResources() ? `<div class="table-wrap"><table><thead><tr><th scope="col">IP</th><th scope="col">到期时间</th><th scope="col">状态</th></tr></thead><tbody>${['192.0.2.18', '192.0.2.42', '198.51.100.26', '203.0.113.17'].map((ip, index) => `<tr><td><code>${ip}</code></td><td>${index < 2 ? '5 天内' : '30 天后'}</td><td><span class="status-pill ${index < 2 ? 'expiring' : ''}"><i></i>${index < 2 ? '即将到期' : '可用'}</span></td></tr>`).join('')}</tbody></table></div>` : '<div class="empty"><strong>暂无静态 IP</strong></div>'}</section>`,
    wallet: () => `<section class="panel"><div class="section-header"><div><h2>钱包流水</h2><p>可用余额 ${balances()[2].number} CNY</p></div></div>${hasResources() ? '<div class="table-wrap"><table><thead><tr><th scope="col">类型</th><th scope="col">金额（CNY）</th><th scope="col">变动后余额（CNY）</th></tr></thead><tbody><tr><td>购买动态流量</td><td>−113.50</td><td>286.50</td></tr><tr><td>运营充值</td><td>+400.00</td><td>400.00</td></tr></tbody></table></div>' : '<div class="empty"><strong>暂无钱包流水</strong></div>'}</section>`,
    traffic: () => `<section class="panel"><div class="section-header"><div><h2>动态流量套餐</h2><p>以 GB 计费，长期有效。</p></div></div><div class="offer-list">${[5, 50, 200].map(size => `<div><strong>${size} GB</strong><span>示例档位</span></div>`).join('')}</div></section>`,
    static: () => `<section class="panel"><div class="section-header"><div><h2>静态 IP 报价</h2><p>示例报价 · 此预览止于商品展示。</p></div></div><div class="offer-list">${['美国', '日本', '新加坡'].map(region => `<div><strong>${region}</strong><span>30 天</span></div>`).join('')}</div></section>`,
  }
  return `<div class="page-heading"><div><h1>${pages[page].label}</h1>${['traffic', 'static'].includes(page) ? '<p>商品预览 · 示例数据</p>' : ''}</div>${link('overview', '返回概览', 'left')}</div>${panels[page]()}`
}

function render() {
  document.querySelector('#navigation').innerHTML = navigation.map(([label, items]) => `<div class="nav-group">${label ? `<p class="nav-label">${label}</p>` : ''}${items.map(([id, glyph, title]) => `<a class="nav-link" title="${title}" href="#${id}" data-page="${id}" ${page === id ? 'aria-current="page"' : ''}>${icon(glyph)}<span class="side-label">${title}</span></a>`).join('')}</div>`).join('')
  document.querySelector('#breadcrumb').textContent = pages[page].label
  document.querySelector('#tenant-name').textContent = hasResources() ? '星河工作室' : '新租户'
  document.querySelector('#avatar').textContent = hasResources() ? '星' : '新'
  document.querySelector('#variant-name').textContent = variant.name
  document.querySelectorAll('[data-scenario]').forEach(button => button.setAttribute('aria-pressed', String(button.dataset.scenario === scenario)))
  main.innerHTML = page === 'overview' ? overview() : relatedPage()
}

function selectPreview() {
  const url = new URL(location.href)
  url.searchParams.set('variant', variant.id)
  url.searchParams.set('scenario', scenario)
  url.hash = 'overview'
  try { history.replaceState(null, '', url) } catch { location.hash = 'overview' }
  page = 'overview'
  render()
}

function shiftVariant(delta) {
  const index = variants.indexOf(variant)
  variant = variants[(index + delta + variants.length) % variants.length]
  selectPreview()
}

function showDialog(title, body, description = '') {
  if (!dialog.open) dialogTrigger = document.activeElement
  dialog.innerHTML = `<div class="dialog-head"><div><h2 id="dialog-title">${title}</h2>${description ? `<p>${description}</p>` : ''}</div><button class="icon-button" data-close aria-label="关闭">${icon('close')}</button></div>${body}`
  if (!dialog.open) dialog.showModal()
}

function createAccount() {
  if (accounts().length >= 10) {
    showDialog('已达到子账号上限', '<p>每个租户最多可创建 10 个代理子账号。</p>')
    return
  }
  showDialog('创建代理子账号', `<form id="create-account"><div class="form-fields">
    <label>子账号名<input name="name" required pattern="[a-z][a-z0-9]{3,7}" placeholder="例如 test01" autocomplete="off" autofocus><small>4–8 位，以小写字母开头，可包含数字。</small></label>
    <label>密码<input name="password" type="password" required pattern="(?=.*[A-Za-z])(?=.*[0-9])[A-Za-z0-9]{6,12}" placeholder="设置 SOCKS5 接入密码" autocomplete="new-password"><small>6–12 位，须同时包含英文字母和数字。请使用测试密码。</small></label>
    <label>备注 <span>选填</span><input name="remark" maxlength="512" placeholder="例如 测试环境"></label>
    <label>累计用量上限（GB） <span>选填</span><input name="limit" inputmode="decimal" pattern="[0-9]+([.][0-9]{1,9})?" placeholder="留空表示不限"></label>
    <p class="form-error" id="form-error" role="alert" hidden></p>
    </div><div class="dialog-actions"><button type="button" class="button" data-fill>填入示例</button><button class="button primary" type="submit">创建子账号</button></div></form>`, '子账号用于动态住宅代理接入。')
}

function showConfiguration(index, created = false) {
  const account = accounts()[index]
  configuration = `协议: SOCKS5\n地址: gateway.example.test\n端口: 1080\n用户名: ${primaryAccount()}.${account.name}\n密码: ${account.password}`
  showDialog('SOCKS5 接入配置', `${!hasResources() ? `<div class="configuration-note">${icon('alert')}当前示例账户无可用流量，购买流量后才能建立连接。</div>` : ''}
    <pre class="config" tabindex="0">${escape(configuration)}</pre><div class="dialog-actions"><span id="copy-status" role="status"></span><button class="button" data-close>完成</button><button class="button primary" data-copy>${icon('copy')}复制接入配置</button></div>`, created ? '子账号已创建。以下为演示配置。' : '将以下配置填入支持 SOCKS5 的客户端。')
  dialog.querySelector('[data-copy]').focus()
}

function syncSidebar() {
  const expanded = mobile.matches ? document.body.classList.contains('menu-open') : !document.body.classList.contains('collapsed')
  document.querySelector('#sidebar-toggle').setAttribute('aria-expanded', String(expanded))
}

document.querySelectorAll('[data-icon]').forEach(element => { element.innerHTML = icon(element.dataset.icon) })
document.querySelector('#sidebar-toggle').addEventListener('click', () => {
  document.body.classList.toggle(mobile.matches ? 'menu-open' : 'collapsed')
  syncSidebar()
})
mobile.addEventListener('change', () => { document.body.classList.remove('menu-open'); syncSidebar() })
window.addEventListener('hashchange', () => {
  const requested = location.hash.slice(1)
  if (!Object.hasOwn(pages, requested) || page === requested) return
  page = requested
  render()
})
dialog.addEventListener('close', () => {
  configuration = undefined
  if (dialogTrigger?.isConnected) dialogTrigger.focus()
  else main.focus()
})
document.addEventListener('keydown', event => {
  if (dialog.open || event.altKey || event.ctrlKey || event.metaKey || event.target.closest('input, select, textarea, [contenteditable]')) return
  if (event.key === 'ArrowLeft' || event.key === 'ArrowRight') {
    event.preventDefault()
    shiftVariant(event.key === 'ArrowLeft' ? -1 : 1)
  }
})
document.addEventListener('click', async event => {
  const button = event.target.closest('button, a[data-page]')
  if (!button) return
  if ('shift' in button.dataset) shiftVariant(Number(button.dataset.shift))
  if ('scenario' in button.dataset) { scenario = button.dataset.scenario; selectPreview() }
  if ('create' in button.dataset) createAccount()
  if ('config' in button.dataset) showConfiguration(Number(button.dataset.config))
  if ('close' in button.dataset) dialog.close()
  if ('fill' in button.dataset) {
    const form = dialog.querySelector('form')
    const next = Array.from({ length: 10 }, (_, index) => `test0${index + 1}`).find(name => !accounts().some(account => account.name === name))
    form.elements.name.value = next
    form.elements.password.value = 'Demo2026'
    form.elements.remark.value = '测试环境'
  }
  if ('copy' in button.dataset) {
    const status = dialog.querySelector('#copy-status')
    let copied = true
    try {
      await navigator.clipboard.writeText(configuration)
    } catch {
      const range = document.createRange()
      range.selectNodeContents(dialog.querySelector('.config'))
      const selection = window.getSelection()
      selection.removeAllRanges()
      selection.addRange(range)
      copied = document.execCommand('copy')
    }
    status.textContent = copied ? '已复制 SOCKS5 接入配置' : '配置已选中，请手动复制'
    if (copied && !copiedScenarios.has(scenario)) {
      copiedScenarios.add(scenario)
      if (variant.id === 'setup' && page === 'overview') render()
    }
  }
  if (!button.dataset.page) return
  event.preventDefault()
  const target = button.dataset.page
  if (target === 'guide') showDialog('接入指南', '<ol class="guide"><li>在代理子账号页创建账密。</li><li>购买动态流量，确保租户余额大于 0。</li><li>把 SOCKS5 地址、端口、用户名和密码填入客户端。</li></ol>')
  else if (target === 'profile') showDialog('账户信息', `<p>主账号名 <code>${primaryAccount()}</code></p><p>角色　租户管理员</p>`)
  else {
    page = target
    location.hash = target
    document.body.classList.remove('menu-open')
    syncSidebar()
    render()
    main.focus()
    window.scrollTo(0, 0)
  }
})
dialog.addEventListener('submit', event => {
  event.preventDefault()
  const fields = new FormData(event.target)
  const error = dialog.querySelector('#form-error')
  if (accounts().some(account => account.name === fields.get('name'))) {
    error.textContent = '该子账号名已存在，请换一个名称。'
    error.hidden = false
    event.target.elements.name.focus()
    return
  }
  accounts().unshift({ name: fields.get('name'), password: fields.get('password'), remark: fields.get('remark'), limit: fields.get('limit'), used: '0.00' })
  render()
  showConfiguration(0, true)
})

document.querySelector('#brand-logo').src = brandLogo
syncSidebar()
render()
