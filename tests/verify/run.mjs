import { execFileSync } from 'node:child_process'
import { randomUUID } from 'node:crypto'
import { mkdirSync, writeFileSync } from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

const root = fileURLToPath(new URL('../../', import.meta.url))
const suite = path.join(root, 'tests/verify')
const runId = randomUUID()
const project = 'proxy-verify-' + runId
const evidence = path.join(root, '.scratch/verify', runId)
mkdirSync(evidence, { recursive: true })
console.log('Evidence:', evidence)

function compose(args, capture = false) {
  return execFileSync('docker', [
    'compose', '-p', project, '-f', 'deploy/test/compose.yaml',
    '-f', 'deploy/test/compose.verify.yaml', ...args,
  ], { cwd: root, encoding: 'utf8', stdio: capture ? ['ignore', 'pipe', 'inherit'] : 'inherit' })
}

function hostPort(service, containerPort) {
  const endpoint = compose(['port', service, String(containerPort)], true).trim()
  const port = /^127\.0\.0\.1:(\d+)$/.exec(endpoint)?.[1]
  if (!port) throw new Error(`Unexpected ${service} address: ${endpoint}`)
  return port
}

const run = { project, startedAt: new Date().toISOString(), result: 'failed' }
try {
  run.commit = execFileSync('git', ['rev-parse', 'HEAD'], { cwd: root, encoding: 'utf8' }).trim()
  run.workingTree = execFileSync('git', ['status', '--porcelain'], { cwd: root, encoding: 'utf8' }).trimEnd()
  run.stage = 'build'
  compose(['build', 'console', 'control', 'migrate'])
  run.stage = 'deploy'
  compose(['up', '-d', '--wait', '--wait-timeout', '90', 'db'])
  compose(['run', '--rm', 'migrate'])
  compose(['up', '-d', '--wait', '--wait-timeout', '120', 'console'])
  run.images = compose(['images'], true).trim()
  run.baseURL = 'https://localhost:' + hostPort('console', 443)
  const supplierURL = 'http://127.0.0.1:' + hostPort('ipcrown', 8080)
  run.stage = 'browser'
  execFileSync(process.execPath, [
    fileURLToPath(import.meta.resolve('@playwright/test/cli')), 'test',
  ], {
    cwd: suite, stdio: 'inherit',
    env: { ...process.env, VERIFY_URL: run.baseURL, VERIFY_OUTPUT: evidence, VERIFY_ID: runId, VERIFY_IPCROWN_URL: supplierURL },
  })
  run.result = 'passed'
} catch (error) {
  run.error = error.message
  console.error(error.message)
  process.exitCode = 1
} finally {
  try {
    compose(['down', '--volumes', '--rmi', 'local'])
    run.cleanedUp = true
  } catch (error) {
    run.cleanedUp = false
    run.result = 'failed'
    run.cleanupError = error.message
    console.error(error.message)
    process.exitCode = 1
  }
  run.finishedAt = new Date().toISOString()
  writeFileSync(path.join(evidence, 'run.json'), JSON.stringify(run, null, 2) + '\n')
}
