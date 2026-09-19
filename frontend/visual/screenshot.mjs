// 目视验证基线截图：起 vite preview，对核心路由截视口图到 visual/screenshots/。
// 用法：pnpm run visual        （先 pnpm run build 或已有 dist）
// 注意：后端不可用时页面呈空态/错误态——本脚本验证的是「布局结构」，数据内容由
// 双端联调覆盖。截图对比可用于重构护航（重构前后各跑一次，逐字节比较）。
import { spawn } from 'node:child_process'
import { mkdirSync } from 'node:fs'
import { chromium } from 'playwright-core'

const PORT = 4173
const BASE = `http://localhost:${PORT}`
const OUT = new URL('./screenshots/', import.meta.url).pathname

// 核心路由（跳过带路径参数的详情页与 404 兜底）。
const ROUTES = [
  '/dashboard',
  '/reviews',
  '/scan',
  '/argus',
  '/sessions',
  '/contributors',
  '/projects/reports',
  '/reports/weekly',
  '/diagnostics',
  '/settings/engine',
  '/settings/model',
  '/settings/ocr',
  '/settings/projects',
  '/settings/platforms',
  '/settings/rules',
  '/settings/global-rules',
  '/settings/agents',
]

const ROUTE_FILE = (r) => (r === '/' ? 'root' : r.replaceAll('/', '_').replace(/^_/, '')) + '.png'

function startPreview() {
  const proc = spawn('./node_modules/.bin/vite', ['preview', '--port', String(PORT), '--strictPort'], {
    cwd: new URL('..', import.meta.url).pathname,
    stdio: 'ignore',
  })
  return proc
}

async function waitForServer(url, timeoutMs = 15000) {
  const deadline = Date.now() + timeoutMs
  while (Date.now() < deadline) {
    try {
      const res = await fetch(url)
      if (res.ok || res.status === 404) return
    } catch { /* not up yet */ }
    await new Promise((r) => setTimeout(r, 300))
  }
  throw new Error(`preview server not reachable at ${url}`)
}

async function main() {
  mkdirSync(OUT, { recursive: true })
  const proc = startPreview()
  try {
    await waitForServer(BASE)
    const browser = await chromium.launch({ channel: 'chrome', headless: true })
    const ctx = await browser.newContext({ viewport: { width: 1440, height: 900 } })
    // 预置登录态（dev-token 与集成测试一致），避免守卫重定向到 /login。
    await ctx.addInitScript(() => localStorage.setItem('token', 'dev-token'))
    const page = await ctx.newPage()

    for (const route of ROUTES) {
      await page.goto(BASE + route, { waitUntil: 'networkidle', timeout: 20000 }).catch(() => {})
      await page.waitForTimeout(600) // 过渡动画/懒加载组件沉淀
      await page.screenshot({ path: OUT + ROUTE_FILE(route) })
      console.log('captured', route, '->', ROUTE_FILE(route))
    }

    await browser.close()
  } finally {
    proc.kill('SIGTERM')
  }
}

main().catch((e) => {
  console.error(e)
  process.exit(1)
})
