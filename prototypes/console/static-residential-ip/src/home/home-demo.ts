// ⚠️ 演示数据：官网首页原型给内部评审用，以下数字、评价和联系方式均为编造或占位，上线前统一替换。
// 价格不在这里：动态起价读运营可调的流量套餐，静态起价读皇冠目录快照，与控制台原型一致。

import type { Component } from 'vue'
import { Aim, DataAnalysis, Goods, Iphone, Monitor, TrendCharts } from '@element-plus/icons-vue'

export const brandName = 'Proxy'

export const telegram = { handle: '@Proxy_Support（占位）', url: 'https://t.me/' }

export const heroStats = [
  { value: 90_000_000, suffix: '+', label: '真人住宅 IP', format: 'wan' },
  { value: 195, suffix: '+', label: '国家和地区', format: 'int' },
  { value: 99.9, suffix: '%', label: '连接成功率', format: 'pct' },
  { value: 0.5, prefix: '< ', suffix: ' s', label: '平均响应', format: 'sec' },
] as const

export type HeroStat = (typeof heroStats)[number]

export function formatStat(stat: HeroStat, value: number) {
  const prefix = 'prefix' in stat ? stat.prefix : ''
  if (stat.format === 'wan') return `${prefix}${Math.round(value / 10_000).toLocaleString('zh-CN')} 万${stat.suffix}`
  if (stat.format === 'pct') return `${prefix}${value.toFixed(1)}${stat.suffix}`
  if (stat.format === 'sec') return `${prefix}${value.toFixed(1)}${stat.suffix}`
  return `${prefix}${Math.round(value)}${stat.suffix}`
}

// 地图与地球上的节点，经纬度为城市近似坐标。
export interface City { name: string, lon: number, lat: number }
export const cities: City[] = [
  { name: '洛杉矶', lon: -118.2, lat: 34.1 },
  { name: '纽约', lon: -74, lat: 40.7 },
  { name: '多伦多', lon: -79.4, lat: 43.7 },
  { name: '墨西哥城', lon: -99.1, lat: 19.4 },
  { name: '圣保罗', lon: -46.6, lat: -23.5 },
  { name: '伦敦', lon: -0.1, lat: 51.5 },
  { name: '法兰克福', lon: 8.7, lat: 50.1 },
  { name: '巴黎', lon: 2.35, lat: 48.9 },
  { name: '迪拜', lon: 55.3, lat: 25.2 },
  { name: '约翰内斯堡', lon: 28, lat: -26.2 },
  { name: '孟买', lon: 72.9, lat: 19.1 },
  { name: '新加坡', lon: 103.8, lat: 1.35 },
  { name: '香港', lon: 114.2, lat: 22.3 },
  { name: '东京', lon: 139.7, lat: 35.7 },
  { name: '首尔', lon: 127, lat: 37.6 },
  { name: '悉尼', lon: 151.2, lat: -33.9 },
]

export const coverage = [
  { region: '北美洲', count: '2,860 万', share: 32 },
  { region: '欧洲', count: '2,240 万', share: 25 },
  { region: '亚太', count: '2,050 万', share: 23 },
  { region: '拉丁美洲', count: '1,120 万', share: 12 },
  { region: '中东与非洲', count: '730 万', share: 8 },
]

export const useCases: { icon: Component, title: string, body: string }[] = [
  { icon: Goods, title: '跨境电商', body: '店铺多账号运营，每个店铺固定一个当地住宅出口，降低关联风险。' },
  { icon: Iphone, title: '社媒矩阵', body: 'TikTok、Instagram、Facebook 账号按国家分配静态 IP，长期稳定登录。' },
  { icon: DataAnalysis, title: '数据采集', body: '动态轮换出口抓取公开数据，按实际流量计费，规模随需伸缩。' },
  { icon: Monitor, title: '广告验证', body: '从目标市场的真实住宅网络查看广告投放效果与落地页。' },
  { icon: TrendCharts, title: '市场调研', body: '按国家和城市观察本地价格、搜索结果与竞品动态。' },
  { icon: Aim, title: '品牌保护', body: '在全球巡查仿冒店铺与侵权内容，及时取证。' },
]

export const advantages = [
  { title: '按字节如实计量', body: '动态流量按总字节 1:1 扣减，1 GB = 10⁹ 字节。用量按天可查、可导出 CSV，每一笔都对得上。' },
  { title: '流量长期有效', body: '买到的流量不设有效期，用完为止。不用为了月底清零而赶着用。' },
  { title: '静态 IP 保号续费', body: '到期前续费，原 IP 不变、从原到期时间顺延。到期前 5 天提醒，单个和批量都能续。' },
]

export const steps = [
  { title: '注册账号', body: '使用手机号注册，开通即有控制台管理员账号。' },
  { title: '联系客服充值', body: `通过 Telegram 联系客服为钱包充值，到账后即可购买。` },
  { title: '购买并接入', body: '购买流量或静态 IP，在控制台复制 SOCKS5 配置开始使用。' },
]

export const testimonials = [
  { name: '林泽', role: '跨境电商 · 运营负责人', quote: '十几个美区店铺每个配一条静态 IP，三个月下来没有一次关联。续费保留原 IP 这点太关键了。' },
  { name: 'Sandy Wu', role: '社媒代运营 · 创始人', quote: '团队 40 多个 TikTok 账号分在 6 个国家，子账号分开管、用量分开看，月底对账特别清楚。' },
  { name: '周启航', role: '数据工程师', quote: '我们每天跑几百 GB 的公开数据采集，切过来之后成功率稳定在 99% 以上，计量和我们自己统计的基本一致。' },
  { name: '陈默', role: '广告投放 · 优化师', quote: '要看各国落地页的真实效果，用住宅出口才看得准。接入就是一行 SOCKS5，十分钟搞定。' },
  { name: 'Kevin Lau', role: '出海 SaaS · CTO', quote: '流量不过期是我们选它的主要原因，买一次大包能用很久，单价也低。' },
  { name: '何蔓', role: '品牌保护 · 项目经理', quote: '海外巡查需要稳定的本地网络，客服响应很快，问题基本当天解决。' },
]

export const faqs = [
  { q: '动态住宅流量会过期吗？', a: '不会。购买的流量长期有效，用完为止。余额用尽后已建立的连接会继续计量，后续到账的流量先抵扣这部分。' },
  { q: '流量怎么计算？', a: '按代理连接的总字节 1:1 扣减，1 GB = 1,000,000,000 字节。控制台按天展示用量，可按子账号筛选并导出 CSV。' },
  { q: '静态住宅 IP 到期后还能用原来的 IP 吗？', a: '到期前手动续费即可保留原 IP，有效期从原到期时间顺延，支持单个和批量续费。到期前 5 天控制台会提醒。到期后需要重新购买。' },
  { q: '可以给团队成员分配不同的代理账号吗？', a: '可以。每个账户最多创建 10 个代理子账号，各自设置密码、备注和累计用量上限，共用账户流量余额。' },
  { q: '如何充值？支持哪些付款方式？', a: `注册后通过 Telegram（${'@Proxy_Support'}）联系客服，由客服为钱包充值。到账后从钱包余额购买商品。需要发票也可联系客服。` },
  { q: '支持哪些接入协议？', a: '动态住宅通过 SOCKS5 隧道接入，用户名格式为「主账号名.子账号名」，可追加会话标识保持同一出口。静态住宅 IP 交付后直接提供 IP、端口和账密。' },
  { q: '能在中国大陆使用吗？', a: '不能。所有代理产品均不支持在中国大陆网络环境下使用，请在境外网络或服务器上使用。' },
]

// 官网公开的接入示例使用示例账号，真实网关地址与账密在登录后的控制台查看。
export const sample = {
  host: 'gw.example-proxy.com',
  port: 1080,
  username: 'acme.user01',
  password: 'Passw0rd',
}
