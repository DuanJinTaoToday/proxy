// 由 scripts/gen-catalog.ps1 生成，勿手改。来源：皇冠 IPCrown 正式 API 目录快照（北京时间 2026-11-21 20:40）。
// 独享 cid=200001，共享 cid=28。yuan 为供应商目录报价（人民币元），已去掉 0 元的 test2h 测试卡。
// 共享库存随项目 pid 变化，快照只含不指定项目及 pid 8/9/10。

export interface SupplierPrice { code: string, label: string, yuan: number }
export type ProjectKey = 'none' | 8 | 9 | 10
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

export const catalogQueriedAt = '2026-11-21 20:40'

export const supplierProjects: { pid: ProjectKey, label: string }[] = [
  { pid: 'none', label: '不指定用途' },
  { pid: 8, label: 'TikTok' },
  { pid: 9, label: 'Sora' },
  { pid: 10, label: 'ChatGPT' },
]

export const supplierNodes: SupplierNode[] = [
  { nodeId: 3, code: 'US', country: '美国', province: '纽约', continent: '北美洲', isp: '普通', line: '住宅原生',
    exclusive: { stock: 142, prices: [{ code: 'yp1', label: '月卡（30天）', yuan: 8 }, { code: 'yp3', label: '季卡（90天）', yuan: 24 }, { code: 'yp6', label: '半年卡（180天）', yuan: 48 }, { code: 'np', label: '年卡（360天）', yuan: 96 }] },
    shared: { stock: { none: 33, 8: 12, 9: 50, 10: 47 }, prices: [{ code: 'yp1', label: '月卡（30天）', yuan: 25 }, { code: 'yp3', label: '季卡（90天）', yuan: 75 }, { code: 'yp6', label: '半年卡（180天）', yuan: 150 }, { code: 'np', label: '年卡（360天）', yuan: 300 }, { code: 'rp', label: '日卡（1天）', yuan: 2 }, { code: 'zp', label: '周卡（7天）', yuan: 4 }] } },
  { nodeId: 4, code: 'US', country: '美国', province: '洛杉矶', continent: '北美洲', isp: '双NTT', line: '住宅原生',
    exclusive: { stock: 306, prices: [{ code: 'yp1', label: '月卡（30天）', yuan: 8 }, { code: 'yp3', label: '季卡（90天）', yuan: 24 }, { code: 'yp6', label: '半年卡（180天）', yuan: 48 }, { code: 'np', label: '年卡（360天）', yuan: 96 }] },
    shared: { stock: { none: 25, 8: 6, 9: 30, 10: 29 }, prices: [{ code: 'yp1', label: '月卡（30天）', yuan: 25 }, { code: 'yp3', label: '季卡（90天）', yuan: 75 }, { code: 'yp6', label: '半年卡（180天）', yuan: 150 }, { code: 'np', label: '年卡（360天）', yuan: 300 }, { code: 'rp', label: '日卡（1天）', yuan: 2 }, { code: 'zp', label: '周卡（7天）', yuan: 4 }] } },
  { nodeId: 5, code: 'US', country: '美国', province: '芝加哥', continent: '北美洲', isp: 'GTT资源', line: '住宅原生',
    exclusive: { stock: 0, prices: [{ code: 'yp1', label: '月卡（30天）', yuan: 8 }, { code: 'yp3', label: '季卡（90天）', yuan: 24 }, { code: 'yp6', label: '半年卡（180天）', yuan: 48 }, { code: 'np', label: '年卡（360天）', yuan: 96 }] },
    shared: { stock: { none: 0, 8: 0, 9: 0, 10: 0 }, prices: [{ code: 'yp1', label: '月卡（30天）', yuan: 25 }, { code: 'yp3', label: '季卡（90天）', yuan: 75 }, { code: 'yp6', label: '半年卡（180天）', yuan: 150 }, { code: 'np', label: '年卡（360天）', yuan: 300 }, { code: 'rp', label: '日卡（1天）', yuan: 2 }, { code: 'zp', label: '周卡（7天）', yuan: 4 }] } },
  { nodeId: 6, code: 'GB', country: '英国', province: undefined, continent: '欧洲', isp: '电信', line: '住宅原生',
    exclusive: { stock: 175, prices: [{ code: 'yp1', label: '月卡（30天）', yuan: 15 }, { code: 'yp3', label: '季卡（90天）', yuan: 45 }, { code: 'yp6', label: '半年卡（180天）', yuan: 90 }, { code: 'np', label: '年卡（360天）', yuan: 180 }] },
    shared: { stock: { none: 27, 8: 24, 9: 29, 10: 28 }, prices: [{ code: 'yp1', label: '月卡（30天）', yuan: 25 }, { code: 'yp3', label: '季卡（90天）', yuan: 75 }, { code: 'yp6', label: '半年卡（180天）', yuan: 150 }, { code: 'np', label: '年卡（360天）', yuan: 300 }, { code: 'rp', label: '日卡（1天）', yuan: 2 }, { code: 'zp', label: '周卡（7天）', yuan: 4 }] } },
  { nodeId: 7, code: 'DE', country: '德国', province: undefined, continent: '欧洲', isp: '电信', line: '住宅原生',
    exclusive: { stock: 171, prices: [{ code: 'yp1', label: '月卡（30天）', yuan: 16 }, { code: 'yp3', label: '季卡（90天）', yuan: 48 }, { code: 'yp6', label: '半年卡（180天）', yuan: 96 }, { code: 'np', label: '年卡（360天）', yuan: 192 }] },
    shared: { stock: { none: 29, 8: 31, 9: 32, 10: 32 }, prices: [{ code: 'yp1', label: '月卡（30天）', yuan: 25 }, { code: 'yp3', label: '季卡（90天）', yuan: 75 }, { code: 'yp6', label: '半年卡（180天）', yuan: 150 }, { code: 'np', label: '年卡（360天）', yuan: 300 }, { code: 'rp', label: '日卡（1天）', yuan: 2 }, { code: 'zp', label: '周卡（7天）', yuan: 4 }] } },
  { nodeId: 8, code: 'FR', country: '法国', province: undefined, continent: '欧洲', isp: '电信', line: '住宅原生',
    exclusive: { stock: 0, prices: [{ code: 'yp1', label: '月卡（30天）', yuan: 20 }, { code: 'yp3', label: '季卡（90天）', yuan: 60 }, { code: 'yp6', label: '半年卡（180天）', yuan: 120 }, { code: 'np', label: '年卡（360天）', yuan: 240 }] },
    shared: { stock: { none: 0, 8: 0, 9: 0, 10: 0 }, prices: [{ code: 'yp1', label: '月卡（30天）', yuan: 25 }, { code: 'yp3', label: '季卡（90天）', yuan: 75 }, { code: 'yp6', label: '半年卡（180天）', yuan: 150 }, { code: 'np', label: '年卡（360天）', yuan: 300 }, { code: 'rp', label: '日卡（1天）', yuan: 2 }, { code: 'zp', label: '周卡（7天）', yuan: 4 }] } },
  { nodeId: 9, code: 'ID', country: '印度尼西亚', province: '雅加达', continent: '亚洲', isp: '电信', line: '住宅原生',
    exclusive: { stock: 172, prices: [{ code: 'yp1', label: '月卡（30天）', yuan: 24 }, { code: 'yp3', label: '季卡（90天）', yuan: 72 }, { code: 'yp6', label: '半年卡（180天）', yuan: 144 }, { code: 'np', label: '年卡（360天）', yuan: 288 }] },
    shared: { stock: { none: 20, 8: 2, 9: 29, 10: 29 }, prices: [{ code: 'yp1', label: '月卡（30天）', yuan: 25 }, { code: 'yp3', label: '季卡（90天）', yuan: 75 }, { code: 'yp6', label: '半年卡（180天）', yuan: 150 }, { code: 'np', label: '年卡（360天）', yuan: 300 }, { code: 'rp', label: '日卡（1天）', yuan: 2 }, { code: 'zp', label: '周卡（7天）', yuan: 4 }] } },
  { nodeId: 10, code: 'TW', country: '中国台湾', province: undefined, continent: '亚洲', isp: '电信', line: '住宅原生',
    exclusive: { stock: 163, prices: [{ code: 'yp1', label: '月卡（30天）', yuan: 28 }, { code: 'yp3', label: '季卡（90天）', yuan: 84 }, { code: 'yp6', label: '半年卡（180天）', yuan: 168 }, { code: 'np', label: '年卡（360天）', yuan: 336 }] },
    shared: { stock: { none: 27, 8: 19, 9: 27, 10: 27 }, prices: [{ code: 'yp1', label: '月卡（30天）', yuan: 25 }, { code: 'yp3', label: '季卡（90天）', yuan: 75 }, { code: 'yp6', label: '半年卡（180天）', yuan: 150 }, { code: 'np', label: '年卡（360天）', yuan: 300 }, { code: 'rp', label: '日卡（1天）', yuan: 2 }, { code: 'zp', label: '周卡（7天）', yuan: 4 }] } },
  { nodeId: 11, code: 'JP', country: '日本', province: '东京', continent: '亚洲', isp: 'DODO K.K.,JP', line: '住宅原生',
    exclusive: { stock: 146, prices: [{ code: 'yp1', label: '月卡（30天）', yuan: 22 }, { code: 'yp3', label: '季卡（90天）', yuan: 66 }, { code: 'yp6', label: '半年卡（180天）', yuan: 132 }, { code: 'np', label: '年卡（360天）', yuan: 264 }] },
    shared: { stock: { none: 40, 8: 25, 9: 44, 10: 43 }, prices: [{ code: 'yp1', label: '月卡（30天）', yuan: 25 }, { code: 'yp3', label: '季卡（90天）', yuan: 75 }, { code: 'yp6', label: '半年卡（180天）', yuan: 150 }, { code: 'np', label: '年卡（360天）', yuan: 300 }, { code: 'rp', label: '日卡（1天）', yuan: 2 }, { code: 'zp', label: '周卡（7天）', yuan: 4 }] } },
  { nodeId: 12, code: 'VN', country: '越南', province: '胡志明市', continent: '亚洲', isp: 'spt-vn', line: '住宅原生',
    exclusive: { stock: 0, prices: [{ code: 'yp1', label: '月卡（30天）', yuan: 27 }, { code: 'yp3', label: '季卡（90天）', yuan: 81 }, { code: 'yp6', label: '半年卡（180天）', yuan: 162 }, { code: 'np', label: '年卡（360天）', yuan: 324 }] },
    shared: { stock: { none: 0, 8: 0, 9: 0, 10: 0 }, prices: [{ code: 'yp1', label: '月卡（30天）', yuan: 25 }, { code: 'yp3', label: '季卡（90天）', yuan: 75 }, { code: 'yp6', label: '半年卡（180天）', yuan: 150 }, { code: 'np', label: '年卡（360天）', yuan: 300 }, { code: 'rp', label: '日卡（1天）', yuan: 2 }, { code: 'zp', label: '周卡（7天）', yuan: 4 }] } },
  { nodeId: 13, code: 'KR', country: '韩国', province: '首尔', continent: '亚洲', isp: '世宗电信', line: '住宅原生',
    exclusive: { stock: 145, prices: [{ code: 'yp1', label: '月卡（30天）', yuan: 23 }, { code: 'yp3', label: '季卡（90天）', yuan: 69 }, { code: 'yp6', label: '半年卡（180天）', yuan: 138 }, { code: 'np', label: '年卡（360天）', yuan: 276 }] },
    shared: { stock: { none: 17, 8: 24, 9: 29, 10: 28 }, prices: [{ code: 'yp1', label: '月卡（30天）', yuan: 25 }, { code: 'yp3', label: '季卡（90天）', yuan: 75 }, { code: 'yp6', label: '半年卡（180天）', yuan: 150 }, { code: 'np', label: '年卡（360天）', yuan: 300 }, { code: 'rp', label: '日卡（1天）', yuan: 2 }, { code: 'zp', label: '周卡（7天）', yuan: 4 }] } },
  { nodeId: 14, code: 'HK', country: '中国香港', province: undefined, continent: '亚洲', isp: 'pccw', line: '住宅原生',
    exclusive: { stock: 202, prices: [{ code: 'yp1', label: '月卡（30天）', yuan: 18 }, { code: 'yp3', label: '季卡（90天）', yuan: 54 }, { code: 'yp6', label: '半年卡（180天）', yuan: 108 }, { code: 'np', label: '年卡（360天）', yuan: 216 }] },
    shared: { stock: { none: 27, 8: 12, 9: 28, 10: 29 }, prices: [{ code: 'yp1', label: '月卡（30天）', yuan: 25 }, { code: 'yp3', label: '季卡（90天）', yuan: 75 }, { code: 'yp6', label: '半年卡（180天）', yuan: 150 }, { code: 'np', label: '年卡（360天）', yuan: 300 }, { code: 'rp', label: '日卡（1天）', yuan: 2 }, { code: 'zp', label: '周卡（7天）', yuan: 4 }] } },
  { nodeId: 15, code: 'AU', country: '澳大利亚', province: undefined, continent: '大洋洲', isp: '电信', line: '住宅原生',
    exclusive: { stock: 0, prices: [{ code: 'yp1', label: '月卡（30天）', yuan: 25 }, { code: 'yp3', label: '季卡（90天）', yuan: 75 }, { code: 'yp6', label: '半年卡（180天）', yuan: 150 }, { code: 'np', label: '年卡（360天）', yuan: 300 }] },
    shared: { stock: { none: 0, 8: 0, 9: 0, 10: 0 }, prices: [{ code: 'yp1', label: '月卡（30天）', yuan: 25 }, { code: 'yp3', label: '季卡（90天）', yuan: 75 }, { code: 'yp6', label: '半年卡（180天）', yuan: 150 }, { code: 'np', label: '年卡（360天）', yuan: 300 }, { code: 'rp', label: '日卡（1天）', yuan: 2 }, { code: 'zp', label: '周卡（7天）', yuan: 4 }] } },
  { nodeId: 16, code: 'ES', country: '西班牙', province: undefined, continent: '欧洲', isp: '电信', line: '住宅原生',
    exclusive: { stock: 0, prices: [{ code: 'yp1', label: '月卡（30天）', yuan: 18 }, { code: 'yp3', label: '季卡（90天）', yuan: 54 }, { code: 'yp6', label: '半年卡（180天）', yuan: 108 }, { code: 'np', label: '年卡（360天）', yuan: 216 }] },
    shared: { stock: { none: 0, 8: 0, 9: 0, 10: 0 }, prices: [{ code: 'yp1', label: '月卡（30天）', yuan: 25 }, { code: 'yp3', label: '季卡（90天）', yuan: 75 }, { code: 'yp6', label: '半年卡（180天）', yuan: 150 }, { code: 'np', label: '年卡（360天）', yuan: 300 }, { code: 'rp', label: '日卡（1天）', yuan: 2 }, { code: 'zp', label: '周卡（7天）', yuan: 4 }] } },
  { nodeId: 17, code: 'IT', country: '意大利', province: undefined, continent: '欧洲', isp: '电信', line: '住宅原生',
    exclusive: { stock: 0, prices: [{ code: 'yp1', label: '月卡（30天）', yuan: 18 }, { code: 'yp3', label: '季卡（90天）', yuan: 54 }, { code: 'yp6', label: '半年卡（180天）', yuan: 108 }, { code: 'np', label: '年卡（360天）', yuan: 216 }] },
    shared: { stock: { none: 0, 8: 0, 9: 0, 10: 0 }, prices: [{ code: 'yp1', label: '月卡（30天）', yuan: 25 }, { code: 'yp3', label: '季卡（90天）', yuan: 75 }, { code: 'yp6', label: '半年卡（180天）', yuan: 150 }, { code: 'np', label: '年卡（360天）', yuan: 300 }, { code: 'rp', label: '日卡（1天）', yuan: 2 }, { code: 'zp', label: '周卡（7天）', yuan: 4 }] } },
  { nodeId: 18, code: 'BR', country: '巴西', province: undefined, continent: '南美洲', isp: '电信', line: '住宅原生',
    exclusive: { stock: 0, prices: [{ code: 'yp1', label: '月卡（30天）', yuan: 30 }, { code: 'yp3', label: '季卡（90天）', yuan: 90 }, { code: 'yp6', label: '半年卡（180天）', yuan: 180 }, { code: 'np', label: '年卡（360天）', yuan: 360 }] },
    shared: { stock: { none: 0, 8: 0, 9: 0, 10: 0 }, prices: [{ code: 'yp1', label: '月卡（30天）', yuan: 25 }, { code: 'yp3', label: '季卡（90天）', yuan: 75 }, { code: 'yp6', label: '半年卡（180天）', yuan: 150 }, { code: 'np', label: '年卡（360天）', yuan: 300 }, { code: 'rp', label: '日卡（1天）', yuan: 2 }, { code: 'zp', label: '周卡（7天）', yuan: 4 }] } },
  { nodeId: 19, code: 'AE', country: '迪拜', province: undefined, continent: '中东', isp: '电信', line: '住宅原生',
    exclusive: { stock: 0, prices: [{ code: 'yp1', label: '月卡（30天）', yuan: 60 }, { code: 'yp3', label: '季卡（90天）', yuan: 180 }, { code: 'yp6', label: '半年卡（180天）', yuan: 360 }, { code: 'np', label: '年卡（360天）', yuan: 720 }] },
    shared: { stock: { none: 0, 8: 0, 9: 0, 10: 0 }, prices: [{ code: 'yp1', label: '月卡（30天）', yuan: 25 }, { code: 'yp3', label: '季卡（90天）', yuan: 75 }, { code: 'yp6', label: '半年卡（180天）', yuan: 150 }, { code: 'np', label: '年卡（360天）', yuan: 300 }, { code: 'rp', label: '日卡（1天）', yuan: 2 }, { code: 'zp', label: '周卡（7天）', yuan: 4 }] } },
  { nodeId: 20, code: 'NL', country: '荷兰', province: undefined, continent: '欧洲', isp: '电信', line: '住宅原生',
    exclusive: { stock: 0, prices: [{ code: 'yp1', label: '月卡（30天）', yuan: 18 }, { code: 'yp3', label: '季卡（90天）', yuan: 54 }, { code: 'yp6', label: '半年卡（180天）', yuan: 108 }, { code: 'np', label: '年卡（360天）', yuan: 216 }] },
    shared: { stock: { none: 0, 8: 0, 9: 0, 10: 0 }, prices: [{ code: 'yp1', label: '月卡（30天）', yuan: 25 }, { code: 'yp3', label: '季卡（90天）', yuan: 75 }, { code: 'yp6', label: '半年卡（180天）', yuan: 150 }, { code: 'np', label: '年卡（360天）', yuan: 300 }, { code: 'rp', label: '日卡（1天）', yuan: 2 }, { code: 'zp', label: '周卡（7天）', yuan: 4 }] } },
  { nodeId: 21, code: 'US', country: '美国', province: '洛杉矶', continent: '北美洲', isp: '普通', line: '住宅原生',
    exclusive: { stock: 96, prices: [{ code: 'yp1', label: '月卡（30天）', yuan: 8 }, { code: 'yp3', label: '季卡（90天）', yuan: 24 }, { code: 'yp6', label: '半年卡（180天）', yuan: 48 }, { code: 'np', label: '年卡（360天）', yuan: 96 }] },
    shared: { stock: { none: 0, 8: 0, 9: 0, 10: 0 }, prices: [{ code: 'yp1', label: '月卡（30天）', yuan: 25 }, { code: 'yp3', label: '季卡（90天）', yuan: 75 }, { code: 'yp6', label: '半年卡（180天）', yuan: 150 }, { code: 'np', label: '年卡（360天）', yuan: 300 }, { code: 'rp', label: '日卡（1天）', yuan: 2 }, { code: 'zp', label: '周卡（7天）', yuan: 4 }] } },
  { nodeId: 23, code: 'PH', country: '菲律宾', province: '马尼拉', continent: '亚洲', isp: '电信', line: '住宅原生',
    exclusive: { stock: 108, prices: [{ code: 'yp1', label: '月卡（30天）', yuan: 24 }, { code: 'yp3', label: '季卡（90天）', yuan: 72 }, { code: 'yp6', label: '半年卡（180天）', yuan: 144 }, { code: 'np', label: '年卡（360天）', yuan: 288 }] },
    shared: { stock: { none: 29, 8: 11, 9: 30, 10: 29 }, prices: [{ code: 'yp1', label: '月卡（30天）', yuan: 25 }, { code: 'yp3', label: '季卡（90天）', yuan: 75 }, { code: 'yp6', label: '半年卡（180天）', yuan: 150 }, { code: 'np', label: '年卡（360天）', yuan: 300 }, { code: 'rp', label: '日卡（1天）', yuan: 2 }, { code: 'zp', label: '周卡（7天）', yuan: 4 }] } },
]
