import { createHash } from 'node:crypto'
import { lookup } from 'node:dns/promises'
import { createServer } from 'node:http'

const exclusive = 200001
const shared = 28
const sharedPrices = new Map([[0, 8], [8, 9], [9, 10], [10, 11]])
const termDays = new Map([['rp', 1], ['zp', 7], ['yp1', 30], ['yp3', 90], ['yp6', 180]])
const catalogFaults = new Set()
const catalogCalls = new Map()
const allocations = new Map()
const orders = new Map()
const pendingRenewals = new Map()
const rejectNextPurchaseNodes = new Set()
const delayedRenewalIDs = new Set()
const renewalCalls = new Map()
const exchangeCalls = new Map()
const exchangeOutcomes = new Map()
const sequenceByNode = new Map()
const day = 24 * 60 * 60 * 1000

let usPrice = 10
let usStock = 6
let detailAvailable = true
let nextOrderId = 7500
let nextPartialProxyPorts = []

function price(term, amount) {
  return { price_type: term, buytime: Math.floor(termDays.get(term) / 30), price: amount }
}

function catalog(cid, pid = 0) {
  if (cid === exclusive) {
    return [
      { id: 101, title: 'United States', continent: 'North America', line: 'Residential', isp: '独享', max: usStock, prices: [price('yp1', usPrice), price('yp3', 27)] },
      { id: 111, title: 'Canada', continent: 'North America', line: 'Residential', isp: '独享', max: 5, prices: [price('yp1', 9)] },
      { id: 201, title: 'Japan', continent: 'Asia', line: 'ISP', isp: '独享', max: 6, prices: [price('yp1', 15), price('yp6', 80)] },
    ]
  }
  const monthly = sharedPrices.get(pid)
  if (cid === shared && monthly !== undefined) {
    return [
      { id: 301, title: 'Singapore', continent: 'Asia', line: 'Residential', isp: '共享', max: 5, prices: [price('rp', 1), price('zp', 3), price('yp1', monthly)] },
    ]
  }
  return []
}

function supplierTime(ms) {
  return new Date(ms + 8 * 60 * 60 * 1000).toISOString().slice(0, 19).replace('T', ' ')
}

function allocate(node, days, position, proxyPort) {
  const sequence = (sequenceByNode.get(node.id) ?? 0) + 1
  sequenceByNode.set(node.id, sequence)
  const id = node.id * 100 + sequence
  const expiresAt = Date.now() + day * (node.id === 201 && position === 0 ? 3 : days)
  const prefix = node.id === 301 ? '203.0.113' : node.id === 201 ? '198.51.100' : '192.0.2'
  const suffix = node.id === 301 ? sequence : node.id + sequence - 1
  const item = {
    id, nid: node.id, ip: proxyPort ? '127.0.0.1' : `${prefix}.${suffix}`, port: proxyPort ?? 43000,
    p_user: 'fixture-user', p_pass: 'fixture-pass', stoptime: supplierTime(expiresAt), status: 1,
  }
  allocations.set(id, item)
  return item
}

function place({ cid, pid, time, list }) {
  const unavailable = { code: -5, msg: 'Fixture stock or specification unavailable' }
  if (!Array.isArray(list) || list.length === 0) return unavailable
  const nodes = new Map(catalog(cid, pid).map(node => [node.id, node]))
  const selections = new Map()
  for (const { nid, number } of list) {
    const node = nodes.get(nid)
    if (!node || selections.has(node) || !Number.isInteger(number) || number <= 0 ||
        number > node.max || !node.prices.some(price => price.price_type === time)) return unavailable
    selections.set(node, node.id === 101 && number === 3 ? 2 : number)
  }
  for (const { nid } of list) {
    if (rejectNextPurchaseNodes.delete(nid)) return unavailable
  }
  const isPartial = list.some(({ nid, number }) => nid === 101 && number === 3)
  const proxyPorts = isPartial ? nextPartialProxyPorts.splice(0) : []
  const items = [...selections].flatMap(([node, count]) =>
    Array.from({ length: count }, (_, position) =>
      allocate(node, termDays.get(time), position, node.id === 101 ? proxyPorts[position] : undefined)))
  const orderId = nextOrderId++
  orders.set(String(orderId), items)
  return { code: 0, data: { order_id: orderId, list: items } }
}

function renew({ cid, ids, time }) {
  if (!Array.isArray(ids) || !termDays.has(time) || ids.some(id => {
    const item = allocations.get(id)
    return !item || cid !== (item.nid === 301 ? shared : exclusive)
  })) {
    return { code: -5, msg: 'Fixture renewal specification unavailable' }
  }
  if (ids.length === 1 && ids[0] === 20102) return { code: -4, data: [] }
  const results = []
  for (const id of ids) {
    renewalCalls.set(id, (renewalCalls.get(id) ?? 0) + 1)
    const allocation = allocations.get(id)
    const original = Date.parse(allocation.stoptime.replace(' ', 'T') + '+08:00')
    const stoptime = supplierTime(original + termDays.get(time) * day)
    // Item 20102 remains unresolved until the operator records a confirmed failure.
    if (id === 20102) continue
    // Item 20103 is visible in detail only after the test releases the delayed confirmation.
    if (id === 20103 || delayedRenewalIDs.has(id)) {
      pendingRenewals.set(id, stoptime)
      continue
    }
    allocation.stoptime = stoptime
    results.push({ id, stoptime })
  }
  return { code: 0, data: results }
}

async function exchange({ type, ids, list, isrand }, response) {
  const item = Array.isArray(ids) && ids.length === 1 ? allocations.get(ids[0]) : undefined
  if (!item || item.nid === 301 || type !== 'same' || isrand !== 4 ||
      !Array.isArray(list) || list.length !== 1 || list[0].nid !== item.nid || list[0].number !== 1) {
    return json(response, 400, { error: 'Expected one original detail ID and its matching node' })
  }
  exchangeCalls.set(item.id, (exchangeCalls.get(item.id) ?? 0) + 1)
  const outcome = exchangeOutcomes.get(item.id)
  if (outcome === 'failed') return json(response, 200, { code: -5, data: [] })
  const ports = { ready: 1081, unknown: 1082, unreachable: 65000 }
  if (!Object.hasOwn(ports, outcome)) return json(response, 400, { error: 'Expected configured exchange outcome' })
  const { address } = await lookup('ipcrown-socks5', { family: 4 })
  Object.assign(item, { ip: address, port: ports[outcome], proxy_id: 1000000 + item.id })
  if (outcome === 'unknown') {
    response.destroy()
    return
  }
  return json(response, 200, { code: 0, data: [{ ...item, proxyid: item.proxy_id }] })
}

function json(response, status, data) {
  response.writeHead(status, { 'Content-Type': 'application/json' })
  response.end(JSON.stringify(data))
}

function supplierSignature(params) {
  const parts = Object.keys(params).sort().map(key => {
    const value = params[key]
    return `${key}=${typeof value === 'string' ? value : JSON.stringify(value)}`
  })
  parts.push('key=verify-supplier-secret')
  return createHash('md5').update(parts.join('&')).digest('hex').toUpperCase()
}

async function handle(request, response) {
  const path = request.url
  if (request.method === 'GET' && path === '/healthz') return json(response, 200, { ok: true })
  if (request.method !== 'POST') return json(response, 404, { error: 'Unknown fixture route' })
  const chunks = []
  for await (const chunk of request) chunks.push(chunk)
  const body = JSON.parse(Buffer.concat(chunks).toString() || '{}')

  if (path === '/fixture/partial-proxy-ports') {
    const { ports } = body
    if (!Array.isArray(ports) || ![0, 2].includes(ports.length) || new Set(ports).size !== ports.length ||
        ports.some(port => !Number.isInteger(port) || port < 1 || port > 65535)) {
      return json(response, 400, { error: 'Expected zero or two distinct valid SOCKS5 ports' })
    }
    nextPartialProxyPorts = ports
    return json(response, 200, { ok: true })
  }
  if (path === '/fixture/purchases/reject-next') {
    const { node_id } = body
    if (!Number.isSafeInteger(node_id) || node_id <= 0) return json(response, 400, { error: 'Expected a fixture node ID' })
    rejectNextPurchaseNodes.add(node_id)
    return json(response, 200, { ok: true })
  }
  if (path === '/fixture/catalog-control') {
    const { category, purpose = 0, unavailable } = body
    if (![exclusive, shared].includes(category) || ![0, 8, 9, 10].includes(purpose) ||
        (unavailable !== undefined && typeof unavailable !== 'boolean')) {
      return json(response, 400, { error: 'Invalid test catalog specification' })
    }
    const key = String(category) + '/' + purpose
    if (unavailable === true) catalogFaults.add(key)
    if (unavailable === false) catalogFaults.delete(key)
    return json(response, 200, { calls: catalogCalls.get(key) ?? 0 })
  }

  if (path === '/fixture/catalog') {
    if (!Number.isFinite(body.price) || body.price <= 0 || !Number.isSafeInteger(body.stock) || body.stock < 0) {
      return json(response, 400, { error: 'Expected a positive price and nonnegative stock' })
    }
    usPrice = body.price
    usStock = body.stock
    return json(response, 200, { price: usPrice, stock: usStock })
  }
  if (path === '/fixture/detail') {
    if (typeof body.available !== 'boolean') return json(response, 400, { error: 'Expected detail availability' })
    detailAvailable = body.available
    return json(response, 200, { available: detailAvailable })
  }
  const findAllocation = ip => [...allocations.values()].find(item => item.ip === ip)

  if (path === '/fixture/exchanges') {
    const item = body.id === undefined ? findAllocation(body.ip) : allocations.get(body.id)
    if (!item || (body.outcome !== undefined && !['ready', 'failed', 'unknown', 'unreachable'].includes(body.outcome))) {
      return json(response, 400, { error: 'Expected a fixture allocation and optional exchange outcome' })
    }
    if (body.outcome !== undefined) exchangeOutcomes.set(item.id, body.outcome)
    return json(response, 200, { id: item.id, calls: exchangeCalls.get(item.id) ?? 0, renewals: renewalCalls.get(item.id) ?? 0 })
  }

  if (path === '/fixture/renewals/confirm') {
    const ids = body.ip === undefined ? [...pendingRenewals.keys()] : [findAllocation(body.ip)?.id]
    for (const id of ids) {
      if (!pendingRenewals.has(id)) continue
      allocations.get(id).stoptime = pendingRenewals.get(id)
      pendingRenewals.delete(id)
      delayedRenewalIDs.delete(id)
    }
    return json(response, 200, { ok: true })
  }
  if (path === '/fixture/renewals/delay') {
    const allocation = findAllocation(body.ip)
    if (!allocation) return json(response, 400, { error: 'Expected an allocated fixture IP' })
    delayedRenewalIDs.add(allocation.id)
    return json(response, 200, { ok: true })
  }

  const { sign, ...params } = body
  if (request.headers['x-partner-id'] !== 'verify' || sign !== supplierSignature(params)) {
    return json(response, 401, { error: 'Invalid fixture supplier signature' })
  }

  switch (path) {
    case '/open/api/nodes-and-continents': {
      const key = String(params.cid) + '/' + (params.pid ?? 0)
      catalogCalls.set(key, (catalogCalls.get(key) ?? 0) + 1)
      if (catalogFaults.has(key)) return json(response, 503, { error: 'Fixture catalog temporarily unavailable' })
      return json(response, 200, { code: 0, data: { nodes: catalog(params.cid, params.pid) } })
    }
    case '/open/api/order/place':
      return json(response, 200, place(params))
    case '/open/api/order/detail': {
      if (!detailAvailable) return json(response, 503, { error: 'Fixture order detail temporarily unavailable' })
      const items = orders.get(String(params.order_id))
      return json(response, 200, items
        ? { code: 0, data: { order_id: params.order_id, proxy_list: items } }
        : { code: -5, msg: 'Fixture order not found' })
    }
    case '/open/api/order/renew':
      return json(response, 200, renew(params))
    case '/open/api/order/exchange':
      return exchange(params, response)
    default:
      return json(response, 404, { error: 'Unknown fixture route' })
  }
}

createServer((request, response) => {
  handle(request, response).catch(error => {
    console.error(error)
    json(response, 400, { error: 'Invalid fixture request' })
  })
}).listen(8080, '0.0.0.0')
