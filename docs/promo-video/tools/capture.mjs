#!/usr/bin/env node
// 抓取宣传片用的产品截图。
//
// 用法：
//   node docs/promo-video/tools/capture.mjs                # 抓全套
//   node docs/promo-video/tools/capture.mjs platforms      # 只抓某个
//
// 前置：后端 :8081 + 前端 dev server :3000 都在跑，数据库已灌好演示数据
// （scripts/seed-demo.sql）。截图落在 docs/assets/screenshots/。
import { mkdirSync } from 'node:fs'
import { fileURLToPath, pathToFileURL } from 'node:url'
import { dirname, join } from 'node:path'

const HERE = dirname(fileURLToPath(import.meta.url))
const ROOT = join(HERE, '..', '..', '..')          // 仓库根
const OUT = join(ROOT, 'docs', 'assets', 'screenshots')
const BASE = process.env.PROMO_BASE || 'http://localhost:3000'
const TOKEN = process.env.REVIEW_TOKEN || 'dev-token'
const WIDTH = Number(process.env.PROMO_WIDTH || 1920)

// playwright-core 装在 frontend/node_modules 下，脚本在 docs/ 里，
// 裸 import 解析不到——按绝对路径动态导入。
const { chromium } = await import(
  pathToFileURL(join(ROOT, 'frontend', 'node_modules', 'playwright-core', 'index.mjs')).href
)

// 页面清单：id → 路由（reviewId 指向演示数据里那条门禁未通过的评审）
const PAGES = [
  { id: '00-login',        path: '/login',                  noAuth: true },
  { id: '01-dashboard',    path: '/dashboard' },
  { id: '02-reviews-list', path: '/reviews' },
  { id: '03-review-detail', path: '/reviews/01K8A1Q0M3N7PQR8V2X4YB6Z0' },
  { id: '04-review-gate',  path: '/reviews/01K8A1Q0M3N7PQR8V2X4YB6Z0', clip: { x: 0, y: 0, width: 1180, height: 1080 } },
  { id: '05-settings-engine',   path: '/settings/engine' },
  { id: '06-settings-platforms', path: '/settings/platforms' },
  { id: '07-settings-agents',    path: '/settings/agents' },
  { id: '08-settings-rules',     path: '/settings/rules' },
  { id: '09-settings-projects',  path: '/settings/projects' },
  { id: '10-project-reports',    path: '/projects/reports' },
  { id: '11-diagnostics',        path: '/diagnostics' },
  { id: '12-sessions',           path: '/sessions' },
  { id: '13-contributors',       path: '/contributors' },
  { id: '14-weekly-report',      path: '/reports/weekly' },
  { id: '15-scan',               path: '/scan' },
]

const only = process.argv.slice(2)
const targets = only.length ? PAGES.filter((p) => only.includes(p.id) || only.includes(p.id.split('-')[1])) : PAGES

async function main() {
  mkdirSync(OUT, { recursive: true })
  const browser = await chromium.launch({ channel: 'chrome', headless: true })
  const ctx = await browser.newContext({ viewport: { width: WIDTH, height: 1080 }, deviceScaleFactor: 1 })
  // 预置登录态，跳过路由守卫
  await ctx.addInitScript((t) => localStorage.setItem('token', t), TOKEN)
  const page = await ctx.newPage()

  for (const p of targets) {
    // 登录页必须无 token 访问：带着 token 会被路由守卫直接送进 /dashboard
    if (p.noAuth) {
      const anon = await browser.newContext({ viewport: { width: WIDTH, height: 1080 }, deviceScaleFactor: 1 })
      const ap = await anon.newPage()
      await ap.goto(BASE + p.path, { waitUntil: 'networkidle', timeout: 30000 }).catch(() => {})
      await ap.waitForTimeout(1200)
      await ap.screenshot({ path: join(OUT, `${p.id}.png`), fullPage: true })
      console.log('captured', p.id, '->', join(OUT, `${p.id}.png`), '(匿名)')
      await anon.close()
      continue
    }
    await page.goto(BASE + p.path, { waitUntil: 'networkidle', timeout: 30000 }).catch(() => {})
    await page.waitForTimeout(1200)          // 等过渡动画与懒加载沉淀
    const file = join(OUT, `${p.id}.png`)
    if (p.clip) {
      await page.screenshot({ path: file, clip: p.clip })
    } else {
      await page.screenshot({ path: file, fullPage: true })
    }
    console.log('captured', p.id, '->', file)
  }
  await browser.close()
}

main().catch((e) => { console.error(e); process.exit(1) })
