// 检查中英文翻译：两种语言的键必须一致，源码里用到的静态键必须存在。
import { readdirSync, readFileSync, statSync } from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

const src = fileURLToPath(new URL('../src/', import.meta.url))
const locales = ['zh-CN', 'en']
const problems = []

function walk(dir) {
  return readdirSync(dir).flatMap(name => {
    const file = path.join(dir, name)
    return statSync(file).isDirectory() ? walk(file) : [file]
  })
}

function flatten(value, prefix = '') {
  return Object.entries(value).flatMap(([key, child]) =>
    child && typeof child === 'object' ? flatten(child, `${prefix}${key}.`) : [`${prefix}${key}`])
}

const files = walk(src)
const keys = Object.fromEntries(locales.map(locale => [locale, new Set()]))
for (const dir of new Set(files.filter(f => f.endsWith(`${path.sep}locales${path.sep}zh-CN.json`)).map(path.dirname))) {
  const sets = locales.map(locale => {
    const file = path.join(dir, `${locale}.json`)
    const set = new Set(flatten(JSON.parse(readFileSync(file, 'utf8'))))
    set.forEach(key => keys[locale].add(key))
    return [locale, set, path.relative(src, file)]
  })
  for (const [locale, set, file] of sets) {
    for (const [other, otherSet] of sets) {
      if (other === locale) continue
      for (const key of otherSet) if (!set.has(key)) problems.push(`${file}: 缺少 ${other} 中的键 ${key}`)
    }
  }
}

const usage = /\bt\(\s*'([\w.-]+)'/g
for (const file of files.filter(f => /\.(vue|ts)$/.test(f))) {
  for (const [, key] of readFileSync(file, 'utf8').matchAll(usage)) {
    for (const locale of locales) if (!keys[locale].has(key)) problems.push(`${path.relative(src, file)}: ${locale} 缺少键 ${key}`)
  }
}

if (problems.length) {
  console.error(problems.join('\n'))
  process.exit(1)
}
console.log(`i18n ok: ${keys['zh-CN'].size} keys`)
