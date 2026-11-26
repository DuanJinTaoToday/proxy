import { once } from 'node:events'
import { createServer } from 'node:http'
import { pathToFileURL } from 'node:url'

const hosts = {
  website: 'www.rocproxy.localhost',
  console: 'console.rocproxy.localhost',
  support: 'support.rocproxy-chat.localhost',
  unrelated: 'unrelated.localhost',
}
const fixtureToken = 'synthetic-session'
const fixtureOrder = 'synthetic-order'
const installationURL = /^https:\/\/plugin-code\.salesmartly\.com\/js\/project_\d+_\d+_\d+\.js$/
const vendorPolicy = [
  'script-src https://plugin-code.salesmartly.com https://client.salesmartly.com',
  "style-src 'unsafe-inline' https://plugin-code.salesmartly.com",
  'font-src data: https://plugin-code.salesmartly.com',
  'img-src data: https://plugin-code.salesmartly.com https://client.salesmartly.com',
  'media-src https://client.salesmartly.com',
  'connect-src https://widget.salesmartly.com https://api.salesmartly.com https://msg.salesmartly.com https://srz.salesmartly.com',
]

export async function startSalesmartlyFixture({ widgetScript } = {}) {
  if (widgetScript && !installationURL.test(widgetScript)) {
    throw new Error('Expected the official SaleSmartly project installation script URL.')
  }

  const server = createServer((request, response) => {
    const entry = Object.keys(hosts).find(name => request.headers.host === new URL(origins[name]).host)
    if (!entry || request.method !== 'GET') {
      response.writeHead(404).end()
      return
    }
    const url = new URL(request.url, origins[entry])

    if (entry === 'console' && url.pathname === '/v1/tenant/orders') {
      const authorized = request.headers.authorization === 'Bearer ' + fixtureToken
      response.writeHead(authorized ? 200 : 401, {
        'Content-Type': 'application/json',
        'Cache-Control': 'no-store',
      }).end(JSON.stringify(authorized ? { orders: [{ id: fixtureOrder }] } : { error: 'Unauthorized' }))
      return
    }
    if (entry === 'console' && url.pathname === '/session.js') {
      response.writeHead(200, { 'Content-Type': 'application/javascript' })
        .end(`localStorage.setItem('proxy-console.session', '${fixtureToken}')`)
      return
    }
    if (url.pathname !== '/') {
      response.writeHead(404).end()
      return
    }

    response.setHeader('Referrer-Policy', 'no-referrer')
    response.setHeader('Content-Type', 'text/html; charset=utf-8')
    if (entry === 'support') {
      response.setHeader('Content-Security-Policy', [
        "default-src 'none'",
        'frame-ancestors ' + origins.website + ' ' + origins.console,
        "base-uri 'none'",
        ...vendorPolicy,
      ].join('; '))
      response.end(document('客服隔离来源', widgetScript ? `<script src="${widgetScript}"></script>` : ''))
      return
    }

    response.setHeader('Content-Security-Policy', [
      "default-src 'none'",
      "script-src 'self'",
      "connect-src 'self'",
      'frame-src ' + origins.support,
      "frame-ancestors 'none'",
      "base-uri 'none'",
    ].join('; '))
    const target = url.searchParams.get('frame') === 'unrelated' ? origins.unrelated : origins.support
    const content = entry === 'console' ? `<p>${fixtureOrder}</p><script src="/session.js"></script>` : ''
    const titles = { website: 'RocProxy 官网联验', console: 'RocProxy 控制台联验', unrelated: '未授权嵌入来源' }
    response.end(document(titles[entry], content +
      `<iframe title="访客客服" src="${target}" width="560" height="760" referrerpolicy="no-referrer"></iframe>`))
  })

  server.listen(0, '127.0.0.1')
  await once(server, 'listening')
  const origins = Object.fromEntries(Object.entries(hosts)
    .map(([name, host]) => [name, `http://${host}:${server.address().port}`]))
  return {
    origins,
    close: () => new Promise((resolve, reject) => {
      server.close(error => error ? reject(error) : resolve())
    }),
  }
}

function document(title, content) {
  return `<!doctype html><html lang="zh-CN"><meta charset="utf-8"><title>${title}</title>
<body><h1>${title}</h1>${content}</body></html>`
}

if (process.argv[1] && import.meta.url === pathToFileURL(process.argv[1]).href) {
  const widgetScript = process.env.SALESMARTLY_WIDGET_SCRIPT
  if (!widgetScript) throw new Error('Set SALESMARTLY_WIDGET_SCRIPT to the project installation script URL.')
  const fixture = await startSalesmartlyFixture({ widgetScript })
  console.log(JSON.stringify(fixture.origins, null, 2))
  process.once('SIGINT', () => fixture.close())
}
