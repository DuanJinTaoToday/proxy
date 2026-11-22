export const sample = {
  host: 'gateway.example.com',
  port: 1080,
  username: 'acme.user01',
  password: 'example-password',
}

const proxyUrl = `socks5h://${sample.username}:${sample.password}@${sample.host}:${sample.port}`
export const quickCommand = `curl -x ${proxyUrl.replace(sample.username, `${sample.username}-sid-a7k2`)} https://ipinfo.io`

export const snippets = {
  cURL: `curl -x ${proxyUrl} https://ipinfo.io`,
  Python: `import requests

proxy = "${proxyUrl}"
response = requests.get("https://ipinfo.io",
                        proxies={"http": proxy, "https": proxy"})
print(response.json())`,
  'Node.js': `import axios from 'axios'
import { SocksProxyAgent } from 'socks-proxy-agent'

const agent = new SocksProxyAgent('${proxyUrl}')
const { data } = await axios.get('https://ipinfo.io', { httpsAgent: agent })
console.log(data)`,
  Go: `auth := &proxy.Auth{User: "${sample.username}", Password: "${sample.password}"}
dialer, err := proxy.SOCKS5("tcp", "${sample.host}:${sample.port}", auth, proxy.Direct)
if err != nil {
    log.Fatal(err)
}
client := &http.Client{Transport: &http.Transport{Dial: dialer.Dial}}
response, err := client.Get("https://ipinfo.io")
if err != nil {
    log.Fatal(err)
}
defer response.Body.Close()
fmt.Println(response.Status)`,
}

export const codeTabs = ['demo', 'cURL', 'Python', 'Node.js', 'Go'] as const
export type CodeTab = (typeof codeTabs)[number]

export const exits = [
  { sid: 'a7k2', city: 'losAngeles', country: 'US', ip: '192.0.2.16', ms: 312, bytes: 18_432, x: '20%', y: '24%' },
  { sid: 'q9m1', city: 'london', country: 'GB', ip: '198.51.100.24', ms: 268, bytes: 21_904, x: '77%', y: '20%' },
  { sid: 'z3p8', city: 'tokyo', country: 'JP', ip: '203.0.113.91', ms: 187, bytes: 16_220, x: '82%', y: '56%' },
  { sid: 'w2c6', city: 'frankfurt', country: 'DE', ip: '192.0.2.38', ms: 233, bytes: 17_548, x: '24%', y: '68%' },
  { sid: 'h5d0', city: 'saoPaulo', country: 'BR', ip: '198.51.100.50', ms: 401, bytes: 19_876, x: '66%', y: '76%' },
]
export const regions = ['US', 'JP', 'KR', 'GB', 'DE']
export const dailyUsage = [2.8, 4.2, 3.7, 6.3, 4.9, 5.8, 3.4].map((gb, index) => ({
  day: `10-${18 + index}`, gb,
}))
export const subAccounts = [
  { name: 'shop01', usedGb: 14.8, limitGb: 20 },
  { name: 'audit01', usedGb: 6.2, limitGb: 15 },
  { name: 'collect01', usedGb: 10, limitGb: 10 },
]
export const heroPhrases = ['stores', 'social', 'data', 'ads', 'markets']
export const faqTopics = ['expiry', 'metering', 'renewal', 'team', 'topUp', 'protocol', 'regions']
