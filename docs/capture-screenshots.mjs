import { mkdir, readFile } from 'node:fs/promises'
import { dirname, join } from 'node:path'
import { fileURLToPath } from 'node:url'
import { chromium } from 'playwright'

const root = join(dirname(fileURLToPath(import.meta.url)), '..')
const outDir = join(root, 'docs', 'screenshots')
const base = process.env.AISHOW_URL || 'http://127.0.0.1:5173'

async function loadDotEnv() {
  const envPath = join(root, 'backend', '.env')
  try {
    const text = await readFile(envPath, 'utf8')
    for (const raw of text.split(/\r?\n/)) {
      const line = raw.trim()
      if (!line || line.startsWith('#')) continue
      const eq = line.indexOf('=')
      if (eq < 1) continue
      const key = line.slice(0, eq).trim()
      const value = line.slice(eq + 1).trim()
      if (process.env[key] === undefined) process.env[key] = value
    }
  } catch {
    // .env 不入库，没有也没关系
  }
}

await loadDotEnv()

const user = process.env.AISHOW_ADMIN_USER || process.env.AISHOW_USER
const pass = process.env.AISHOW_ADMIN_PASSWORD || process.env.AISHOW_PASSWORD
if (!user || !pass) {
  console.error('请设置 AISHOW_ADMIN_USER / AISHOW_ADMIN_PASSWORD，或先写好 backend/.env')
  process.exit(1)
}

await mkdir(outDir, { recursive: true })

const browser = await chromium.launch({
  channel: process.env.AISHOW_BROWSER || 'msedge',
  headless: true,
})
const page = await browser.newPage({
  viewport: { width: 1440, height: 920 },
  deviceScaleFactor: 1,
})

async function waitReady() {
  await page.waitForLoadState('networkidle').catch(() => {})
  await page.evaluate(() => document.fonts.ready).catch(() => {})
  await page.waitForTimeout(500)
}

async function shot(name) {
  await waitReady()
  const file = join(outDir, `${name}.png`)
  await page.screenshot({ path: file, type: 'png' })
  console.log('wrote', file)
}

await page.goto(`${base}/login`, { waitUntil: 'domcontentloaded' })
await page.waitForSelector('.login, .shell', { timeout: 20000 })
await waitReady()

if (await page.locator('.login').count()) {
  await shot('login')
  await page.locator('input[autocomplete="username"]').fill(user)
  await page.locator('input[autocomplete="current-password"]').fill(pass)
  await page.locator('button[type="submit"]').click()
  await page.waitForSelector('.shell', { timeout: 20000 })
  await waitReady()
} else {
  await page.goto(`${base}/login`, { waitUntil: 'domcontentloaded' })
  await page.waitForTimeout(300)
  await shot('login')
  await page.goto(`${base}/`, { waitUntil: 'domcontentloaded' })
}

await page.goto(`${base}/`, { waitUntil: 'domcontentloaded' })
await page.waitForSelector('.shell')
await shot('dashboard')

for (const [path, name] of [
  ['/studio', 'studio'],
  ['/queue', 'queue'],
  ['/gallery', 'gallery'],
  ['/nodes', 'nodes'],
  ['/users', 'users'],
]) {
  await page.goto(`${base}${path}`, { waitUntil: 'domcontentloaded' })
  await page.waitForSelector('.shell')
  if (name === 'gallery') {
    await page.waitForSelector('.card video, .card img, .empty', { timeout: 15000 }).catch(() => {})
    await page.waitForTimeout(800)
  }
  await shot(name)
}

await browser.close()
console.log('done')
