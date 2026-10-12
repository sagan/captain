#!/usr/bin/env node
// Visual-regression gate for the "glass" status page.
//
// Serves two built copies of the page (reference and candidate) from two local
// ports, each backed by the same fixture server that answers the Captain public
// probe API with byte-identical fictitious JSON (scripts/glass-fixture.json),
// screenshots both under identical conditions (virtual clock paused at the
// fixture instant, fixed viewports, colour schemes and locale) and reports
// pixel differences per image.
//
//   node scripts/glass-diff.mjs --ref <distA> --cand <distB> --out <dir> [--tolerate <n>] [--hide-canvas] [--no-dialog] [--only <variant,...>] [--scenes <scene,...>] [--dom]
//
// --dom also writes a normalised DOM serialisation per screenshot and reports
// the first differing lines, to explain pixel differences between a Vue and a
// React build. It is diagnostic only and never affects the exit status.
//
// Scenes: home (card view, plus the latency dialog unless --no-dialog), list
// (home in list view) and detail (#/instance/<id> with the load and ping charts).
//   node scripts/glass-diff.mjs --ref <distA> --out <dir> --ref-only
//
// Metric (per the 2026-10-02 acceptance): `over8` is the number of pixels where
// any RGB channel differs by more than 8; `mean` is the mean absolute RGB
// channel difference over the whole image. Exit status is non-zero when any
// image has over8 > tolerate (default 0).
import { createServer } from 'node:http'
import { once } from 'node:events'
import { readFile, stat, mkdir, writeFile } from 'node:fs/promises'
import path from 'node:path'
import { fileURLToPath } from 'node:url'
import { chromium } from 'playwright'
import pixelmatch from 'pixelmatch'
import { PNG } from 'pngjs'

const here = path.dirname(fileURLToPath(import.meta.url))
const fixturePath = path.join(here, 'glass-fixture.json')

// ---------------------------------------------------------------- CLI
function parseArgs(argv) {
  const opts = { ref: '', cand: '', out: '', tolerate: 0, refOnly: false, hideCanvas: false, dialog: true, only: null, scenes: ['home', 'list', 'detail'], dom: false }
  for (let i = 0; i < argv.length; i++) {
    const a = argv[i]
    const next = () => { if (i + 1 >= argv.length) throw new Error(`${a} needs a value`); return argv[++i] }
    switch (a) {
      case '--ref': opts.ref = next(); break
      case '--cand': opts.cand = next(); break
      case '--out': opts.out = next(); break
      case '--tolerate': opts.tolerate = Number(next()); break
      case '--ref-only': opts.refOnly = true; break
      case '--hide-canvas': opts.hideCanvas = true; break
      case '--no-dialog': opts.dialog = false; break
      case '--only': opts.only = next().split(',').map(v => v.trim()).filter(Boolean); break
      case '--dom': opts.dom = true; break
      case '--scenes': opts.scenes = next().split(',').map(v => v.trim()).filter(Boolean); break
      case '-h': case '--help': usage(0); break
      default: throw new Error(`unknown argument ${a}`)
    }
  }
  if (!opts.ref || !opts.out || (!opts.refOnly && !opts.cand)) usage(2)
  if (!Number.isFinite(opts.tolerate) || opts.tolerate < 0) throw new Error('--tolerate must be a non-negative number')
  for (const name of opts.scenes) if (!SCENES.includes(name)) throw new Error(`--scenes: unknown scene ${name}; choose from ${SCENES.join(', ')}`)
  for (const name of opts.only ?? []) if (!variants.some(v => v.name === name)) throw new Error(`--only: unknown variant ${name}; choose from ${variants.map(v => v.name).join(', ')}`)
  return opts
}
function usage(code) {
  console.error('usage: glass-diff.mjs --ref <dist> --cand <dist> --out <dir> [--tolerate <n>] [--hide-canvas] [--no-dialog] [--only <variant,...>] [--scenes <scene,...>] [--dom]\n       glass-diff.mjs --ref <dist> --out <dir> --ref-only [--hide-canvas] [--no-dialog] [--only <variant,...>] [--scenes <scene,...>]\n       variants: desktop-light desktop-dark mobile-light mobile-dark\n       scenes: home (card view + latency dialog), list (list view), detail (instance page with load and ping charts)')
  process.exit(code)
}

// ---------------------------------------------------------------- fixture server
const mime = {
  '.html': 'text/html; charset=utf-8', '.js': 'text/javascript; charset=utf-8', '.mjs': 'text/javascript; charset=utf-8',
  '.css': 'text/css; charset=utf-8', '.json': 'application/json; charset=utf-8', '.svg': 'image/svg+xml', '.png': 'image/png',
  '.jpg': 'image/jpeg', '.jpeg': 'image/jpeg', '.webp': 'image/webp', '.gif': 'image/gif', '.ico': 'image/x-icon',
  '.txt': 'text/plain; charset=utf-8', '.woff': 'font/woff', '.woff2': 'font/woff2', '.ttf': 'font/ttf', '.wasm': 'application/wasm',
  '.map': 'application/json', '.webmanifest': 'application/manifest+json',
}

// Window and step mirror captain/internal/http/probe/probe.go rangeOf(): the
// page only ever asks for 1h/24h/7d/30d (see rangeFor in glass/src/captain.ts).
function rangeWindow(range) {
  switch (range) {
    case '24h': return { res: 'm', window: 24 * 3600, step: 60 }
    case '7d': return { res: 'h', window: 7 * 86400, step: 3600 }
    case '30d': return { res: 'h', window: 30 * 86400, step: 3600 }
    case '1y': return { res: 'd', window: 365 * 86400, step: 86400 }
    default: return { res: 'raw', window: 3600, step: 10 }
  }
}

function apiResponse(fixture, url) {
  const { now, snapshot, history, pings } = fixture
  if (url.pathname === '/api/probe') return snapshot
  const m = /^\/api\/probe\/nodes\/(\d+)\/(history|pings)$/.exec(url.pathname)
  if (!m) return null
  const id = m[1], kind = m[2]
  const table = kind === 'history' ? history : pings
  if (!(id in table)) return null
  let { res, window, step } = rangeWindow(url.searchParams.get('range') ?? '')
  // The pings endpoint has no in-memory ring; it uses minute buckets for the 1h window.
  if (kind === 'pings' && res === 'raw') { res = 'm'; step = 60 }
  if (kind === 'history' && res === 'raw') {
    // Raw ring samples are the node's `recent` list (service.Sample shape).
    const node = snapshot.nodes.find(n => String(n.id) === id)
    return { res, points: node?.recent ?? [], from: now - window, to: now, step: snapshot.beat_seconds }
  }
  return { res, points: table[id].filter(p => p.ts >= now - window && p.ts <= now), from: now - window, to: now, step }
}

async function serveDist(dist, fixture) {
  const root = path.resolve(dist)
  if (!(await stat(path.join(root, 'index.html')).catch(() => null))) throw new Error(`${root} has no index.html`)
  const server = createServer(async (req, res) => {
    try {
      const url = new URL(req.url ?? '/', 'http://localhost')
      if (url.pathname.startsWith('/api/')) {
        const body = apiResponse(fixture, url)
        if (body === null) { res.writeHead(404, { 'content-type': 'text/plain' }); res.end('not found'); return }
        res.writeHead(200, { 'content-type': 'application/json; charset=utf-8', 'cache-control': 'no-store' })
        res.end(JSON.stringify(body))
        return
      }
      let rel = decodeURIComponent(url.pathname)
      let file = path.normalize(path.join(root, rel))
      if (!file.startsWith(root)) { res.writeHead(403); res.end(); return }
      let st = await stat(file).catch(() => null)
      if (!st || st.isDirectory()) { file = path.join(root, 'index.html'); st = await stat(file) } // hash router: any path is the app
      const ext = path.extname(file).toLowerCase()
      res.writeHead(200, { 'content-type': mime[ext] ?? 'application/octet-stream', 'content-length': st.size, 'cache-control': 'no-store' })
      res.end(await readFile(file))
    } catch (err) {
      res.writeHead(500, { 'content-type': 'text/plain' }); res.end(String(err))
    }
  })
  server.listen(0, '127.0.0.1')
  await once(server, 'listening')
  const { port } = server.address()
  return { url: `http://127.0.0.1:${port}`, close: () => new Promise(r => server.close(r)) }
}

// ---------------------------------------------------------------- browser
const variants = [
  { name: 'desktop-light', width: 1440, height: 1080, scheme: 'light' },
  { name: 'desktop-dark', width: 1440, height: 1080, scheme: 'dark' },
  { name: 'mobile-light', width: 390, height: 844, scheme: 'light' },
  { name: 'mobile-dark', width: 390, height: 844, scheme: 'dark' },
]

const SCENES = ['home', 'list', 'detail']
// The detail scene opens the first node that has both load history and a ping task.
const DETAIL_NODE_INDEX = 0

// Time is controlled by Playwright's virtual clock (Date, timers, rAF,
// requestIdleCallback and performance.now together), installed before any page
// script runs and paused at the fixture instant. The page then only advances by
// the fixed budgets below, so every run sees the same frames. A plain frozen
// Date.now is NOT enough: kapsule (globe.gl) schedules its digest through a
// lodash debounce that measures elapsed time with Date.now, so under a constant
// clock the globe never gets sized or drawn and the diff passes vacuously.
const VIRTUAL_SETTLE_MS = 2000 // globe digest + 500 ms points/labels transitions
const VIRTUAL_DIALOG_MS = 1000 // ECharts (zrender) paints on rAF
const VIRTUAL_TICK_MS = 100

// Page settings the real page reads from localStorage; nothing is monkey-patched.
function initStorage({ scheme, viewMode }) {
  try {
    localStorage.setItem('themeMode', scheme)
    localStorage.setItem('i18nextLng', 'zh-CN')
    localStorage.setItem('nodeViewMode', viewMode)
  } catch { /* storage unavailable: the page falls back to the snapshot's scheme */ }
}

// Wait, in real time and with the virtual clock still paused, until nothing is
// loading and the DOM has stopped changing. Module chunks and fetches complete
// on the network's schedule; time-based animations (the globe's auto-rotate
// measures elapsed virtual time from the moment globe.gl is constructed) must
// all start from the same virtual instant, so virtual time only advances from
// a quiescent state.
// Playwright's 'networkidle' load state is one-shot per navigation, so requests
// started later (lazy chunks, globe textures, flag images) are tracked here.
function trackRequests(page) {
  const inflight = new Set()
  page.on('request', r => inflight.add(r))
  page.on('requestfinished', r => inflight.delete(r))
  page.on('requestfailed', r => inflight.delete(r))
  return inflight
}
const QUIET_MS = 300 // real time with no request in flight: covers image decode and onload tasks
async function quiesce(page, inflight) {
  const deadline = Date.now() + 30_000
  for (let quietSince = null; ;) {
    if (inflight.size) quietSince = null
    else if (quietSince === null) quietSince = Date.now()
    else if (Date.now() - quietSince >= QUIET_MS) break
    if (Date.now() > deadline) throw new Error(`page never became quiet; ${inflight.size} request(s) in flight`)
    await page.waitForTimeout(50) // real time (Playwright side), not page time
  }
  await page.evaluate(() => document.fonts.ready)
  let last = null
  for (let stable = 0; stable < 3;) {
    const cur = await page.evaluate(() => `${document.querySelectorAll('*').length}:${document.querySelectorAll('canvas').length}:${document.body.innerText.length}`)
    stable = cur === last ? stable + 1 : 0
    last = cur
    await page.waitForTimeout(100)
  }
  if (inflight.size) return quiesce(page, inflight) // the DOM change started new loads
}

// Advance virtual time in fixed ticks from a quiet state, then quiet down and
// run one more fixed tick so anything that finished loading during the advance
// (textures, chart data) is painted by a frame. Every step is unconditional, so
// each run renders the same number of frames from the same virtual instant.
async function advance(page, inflight, ms) {
  await quiesce(page, inflight)
  for (let t = 0; t < ms; t += VIRTUAL_TICK_MS) await page.clock.runFor(Math.min(VIRTUAL_TICK_MS, ms - t))
  await quiesce(page, inflight)
  await page.clock.runFor(VIRTUAL_TICK_MS)
  await quiesce(page, inflight)
}


// Normalised DOM dump for --dom: one line per element/text, attributes sorted.
// Vue's scope hashes (data-v-xxxxxxxx) and the React build's named scopes
// (data-v-name) both become data-v; Vue comment anchors are dropped; generated
// ids are reduced to their prefix; inline CSS variables named after scope
// hashes are renamed.
function dumpDom() {
  const out = []
  const origin = location.origin
  const norm = v => v
    .split(origin).join('<origin>')
    .replace(/\b(reka-[a-z-]+?-)(?:v-\d+|_r_[0-9a-z]+_|«[^»]*»|:[^:]+:)/g, '$1ID')
    .replace(/--v[0-9a-f]{8}\b|--v[0-9a-z]{6,10}\b/g, '--vbind')
  const walk = (node, depth) => {
    if (node.nodeType === 3) {
      const text = node.textContent
      if (text.trim()) out.push(`${'  '.repeat(depth)}"${text.replace(/\s+/g, ' ')}"`)
      else if (text.length) out.push(`${'  '.repeat(depth)}" "`)
      return
    }
    if (node.nodeType !== 1) return
    const el = node
    const attrs = [...el.attributes]
      .filter(a => !/^data-v-/.test(a.name))
      .map(a => a.name === '_echarts_instance_' ? { name: a.name, value: 'ec' } : a)
      .map(a => `${a.name}=${JSON.stringify(norm(a.value))}`)
      .sort()
    if ([...el.attributes].some(a => /^data-v-/.test(a.name))) attrs.unshift('data-v')
    out.push(`${'  '.repeat(depth)}<${el.tagName.toLowerCase()}${attrs.length ? ' ' + attrs.join(' ') : ''}>`)
    if (el.tagName === 'svg' || el.tagName === 'SCRIPT' || el.tagName === 'STYLE') return
    for (const c of el.childNodes) walk(c, depth + 1)
  }
  for (const c of document.body.childNodes) walk(c, 0)
  return out.join('\n')
}

const freezeCSS = `*,*::before,*::after{animation:none!important;transition:none!important;caret-color:transparent!important}
[data-general-card-key="remainingValue"]{visibility:hidden!important}`
const hideCanvasCSS = `canvas{visibility:hidden!important}`

// Playwright's own waits run in an isolated world with native timers, so they
// are unaffected by the paused clock. Page-side rAF/timers only run via advance().
async function settle(page, inflight, ready) {
  await page.locator(ready.selector).first().waitFor({ timeout: 30_000 })
  await advance(page, inflight, VIRTUAL_SETTLE_MS)
  await page.waitForFunction((sel) => {
    if (document.querySelectorAll(sel).length === 0) return false
    if (document.querySelector('.loading-cover')) return false
    const text = document.body.innerText
    return !text.includes('加载中') && !text.includes('Loading...')
  }, ready.selector, { timeout: 30_000, polling: 100 })
}

// Playwright replays its clock log into every new document using the real
// wall-clock gap between the install and pauseAt calls as the initial
// performance.now() ticks. That gap is usually 0 ms but sometimes 1 ms, and
// the globe's auto-rotation is speed x elapsed ticks, so a 1 ms origin shift
// moves the globe by a fraction of a frame. Verify the origin on a blank page
// and rebuild the context until it is exactly zero.
async function newFrozenContext(browser, variant, fixedMs) {
  for (let attempt = 1; ; attempt++) {
    const context = await browser.newContext({
      viewport: { width: variant.width, height: variant.height }, deviceScaleFactor: 1,
      locale: 'zh-CN', timezoneId: 'UTC', colorScheme: variant.scheme, reducedMotion: 'reduce',
      isMobile: variant.width < 600, hasTouch: variant.width < 600,
    })
    await context.clock.install({ time: fixedMs })
    await context.clock.pauseAt(fixedMs)
    const probe = await context.newPage()
    const origin = await probe.evaluate(() => [performance.now(), Date.now()])
    await probe.close()
    if (origin[0] === 0 && origin[1] === fixedMs) return context
    await context.close()
    if (attempt >= 20) throw new Error(`virtual clock origin never settled at 0 (last ${origin[0]} ms)`)
  }
}

// Each scene runs in its own fresh context so one cannot influence another.
// Home keeps the original file names (<variant>.png, <variant>-dialog.png).
function sceneSetup(scene, fixture) {
  switch (scene) {
    case 'list': {
      const name = fixture.snapshot.nodes[0].name
      return { hash: '', viewMode: 'list', ready: { selector: '.app-shell main', text: name }, file: v => `${v}-list` }
    }
    case 'detail': {
      const node = fixture.snapshot.nodes[DETAIL_NODE_INDEX]
      return { hash: `#/instance/${node.id}`, viewMode: 'card', ready: { selector: '[data-load-chart-card]', text: node.name }, file: v => `${v}-detail` }
    }
    default:
      return { hash: '', viewMode: 'card', ready: { selector: '.node-card' }, file: v => v }
  }
}

async function captureScene(browser, base, variant, fixture, opts, outDir, side, scene) {
  const fixedMs = fixture.now * 1000
  const setup = sceneSetup(scene, fixture)
  const context = await newFrozenContext(browser, variant, fixedMs)
  const page = await context.newPage()
  const inflight = trackRequests(page)
  const errors = []
  page.on('pageerror', e => errors.push(String(e)))
  page.on('console', m => { if (m.type() === 'error') errors.push(m.text().slice(0, 200)) })
  // Only the fixture server may be reached; anything else is a page defect for the diff.
  await page.route(/.*/, route => {
    const u = route.request().url()
    if (u.startsWith(base + '/')) return route.continue()
    errors.push(`blocked external request ${u}`)
    return route.abort()
  })
  await page.addInitScript(initStorage, { scheme: variant.scheme, viewMode: setup.viewMode })
  const shots = {}
  const stem = setup.file(variant.name)
  try {
    await page.goto(base + '/' + setup.hash, { waitUntil: 'domcontentloaded' })
    await settle(page, inflight, setup.ready)
    if (setup.ready.text) await page.getByText(setup.ready.text, { exact: true }).first().waitFor({ timeout: 30_000 })
    await page.addStyleTag({ content: freezeCSS + (opts.hideCanvas ? hideCanvasCSS : '') })
    await advance(page, inflight, VIRTUAL_TICK_MS) // paint after the style change
    const main = path.join(outDir, `${stem}-${side}.png`)
    await page.screenshot({ path: main, fullPage: true, animations: 'disabled', caret: 'hide' })
    shots[`${stem}.png`] = main
    if (opts.dom) await writeFile(path.join(outDir, `${stem}-${side}.dom.txt`), await page.evaluate(dumpDom))

    if (scene === 'home' && opts.dialog) {
      // NodeCard.vue: the latency panel is a button labelled "<node name> 延迟监测"
      // (literal, not translated) that emits pingClick → HomeView opens PingMonitorDialog.
      const firstNode = fixture.snapshot.nodes.find(n => (n.ping_tasks ?? []).length > 0)
      const trigger = page.getByRole('button', { name: `${firstNode.name} 延迟监测`, exact: true }).first()
      await trigger.scrollIntoViewIfNeeded()
      await trigger.click()
      const dialog = page.getByRole('dialog')
      await dialog.waitFor({ timeout: 15_000 })
      await advance(page, inflight, VIRTUAL_DIALOG_MS)
      await page.waitForFunction(() => !document.body.innerText.includes('加载中'), null, { timeout: 15_000, polling: 100 })
      const dlg = path.join(outDir, `${stem}-dialog-${side}.png`)
      await dialog.screenshot({ path: dlg, animations: 'disabled', caret: 'hide' })
      shots[`${stem}-dialog.png`] = dlg
      if (opts.dom) await writeFile(path.join(outDir, `${stem}-dialog-${side}.dom.txt`), await page.evaluate(dumpDom))
    }
  } finally {
    await context.close()
  }
  return { shots, errors: errors.map(e => `${scene}: ${e}`) }
}

async function capture(browser, base, variant, fixture, opts, outDir, side) {
  const shots = {}, errors = []
  for (const scene of opts.scenes) {
    const r = await captureScene(browser, base, variant, fixture, opts, outDir, side, scene)
    Object.assign(shots, r.shots)
    errors.push(...r.errors)
  }
  return { shots, errors }
}

// ---------------------------------------------------------------- diff
async function domDiff(outDir, name) {
  const stem = name.replace(/\.png$/, '')
  const read = side => readFile(path.join(outDir, `${stem}-${side}.dom.txt`), 'utf8').then(t => t.split('\n')).catch(() => null)
  const [a, b] = await Promise.all([read('ref'), read('cand')])
  if (!a || !b) return 'missing'
  if (a.join('\n') === b.join('\n')) return 'identical'
  let i = 0
  while (i < a.length && i < b.length && a[i] === b[i]) i++
  return { firstDiffLine: i + 1, ref: a.slice(i, i + 3), cand: b.slice(i, i + 3), refLines: a.length, candLines: b.length }
}

async function readPNG(file) { return PNG.sync.read(await readFile(file)) }

function compare(a, b) {
  const width = Math.max(a.width, b.width), height = Math.max(a.height, b.height)
  const pad = (img) => {
    if (img.width === width && img.height === height) return img.data
    // Pad with a colour no page uses so size mismatches count as differences.
    const out = Buffer.alloc(width * height * 4, 0)
    for (let y = 0; y < height; y++) for (let x = 0; x < width; x++) {
      const o = (y * width + x) * 4
      if (x < img.width && y < img.height) { const i = (y * img.width + x) * 4; out[o] = img.data[i]; out[o + 1] = img.data[i + 1]; out[o + 2] = img.data[i + 2]; out[o + 3] = 255 }
      else { out[o] = 255; out[o + 1] = 0; out[o + 2] = 255; out[o + 3] = 255 }
    }
    return out
  }
  const da = pad(a), db = pad(b)
  let over8 = 0, sum = 0
  for (let i = 0; i < width * height * 4; i += 4) {
    const dr = Math.abs(da[i] - db[i]), dg = Math.abs(da[i + 1] - db[i + 1]), dbl = Math.abs(da[i + 2] - db[i + 2])
    if (dr > 8 || dg > 8 || dbl > 8) over8++
    sum += dr + dg + dbl
  }
  const diff = new PNG({ width, height })
  pixelmatch(da, db, diff.data, width, height, { threshold: 0.05, includeAA: true, diffColor: [255, 0, 0], alpha: 0.3 })
  return { over8, mean: sum / (width * height * 3), diff, size: [a.width, a.height], native_size: [b.width, b.height] }
}

// ---------------------------------------------------------------- main
async function main() {
  const opts = parseArgs(process.argv.slice(2))
  const fixture = JSON.parse(await readFile(fixturePath, 'utf8'))
  const outDir = path.resolve(opts.out)
  await mkdir(outDir, { recursive: true })
  const servers = [await serveDist(opts.ref, fixture)]
  if (!opts.refOnly) servers.push(await serveDist(opts.cand, fixture))
  const browser = await chromium.launch({
    headless: true,
    args: ['--force-device-scale-factor=1', '--font-render-hinting=none', '--disable-lcd-text', '--hide-scrollbars', '--disable-gpu-vsync', '--enable-unsafe-swiftshader'],
  })
  const summary = {}, problems = []
  try {
    for (const variant of variants.filter(v => !opts.only || opts.only.includes(v.name))) {
      // A variant whose images differ is captured once more on both sides: the
      // realistic globe's rotation origin can land a fraction of a frame apart
      // (see newFrozenContext). Only a difference seen twice counts; a passing
      // retry is recorded as `retried` so the noise stays visible.
      for (let attempt = 1; attempt <= 2; attempt++) {
        const ref = await capture(browser, servers[0].url, variant, fixture, opts, outDir, 'ref')
        for (const e of ref.errors) problems.push(`${variant.name} ref: ${e}`)
        if (opts.refOnly) { for (const [name, file] of Object.entries(ref.shots)) summary[name] = { file: path.relative(outDir, file) }; break }
        const cand = await capture(browser, servers[1].url, variant, fixture, opts, outDir, 'cand')
        for (const e of cand.errors) problems.push(`${variant.name} cand: ${e}`)
        let failed = false
        for (const name of Object.keys(ref.shots)) {
          if (!cand.shots[name]) { summary[name] = { size: null, native_size: null, over8: Infinity, mean: Infinity }; failed = true; continue }
          const r = compare(await readPNG(ref.shots[name]), await readPNG(cand.shots[name]))
          const diffFile = path.join(outDir, name.replace(/\.png$/, '-diff.png'))
          await writeFile(diffFile, PNG.sync.write(r.diff))
          const previous = summary[name]
          summary[name] = { size: r.size, native_size: r.native_size, over8: r.over8, mean: r.mean }
          if (attempt > 1) summary[name].retried = { first_over8: previous?.over8 ?? null }
          if (opts.dom) summary[name].dom = await domDiff(outDir, name)
          if (!(r.over8 <= opts.tolerate)) failed = true
        }
        if (!failed) break
      }
    }
  } finally {
    await browser.close()
    await Promise.all(servers.map(s => s.close()))
  }
  const summaryFile = path.join(outDir, opts.refOnly ? 'screenshots.json' : 'summary.json')
  await writeFile(summaryFile, JSON.stringify(summary, null, 2) + '\n')
  console.log(JSON.stringify(summary, null, 2))
  for (const p of problems) console.error('warning:', p)
  if (opts.refOnly) { console.log(`screenshots written to ${outDir}`); return }
  const failing = Object.entries(summary).filter(([, v]) => !(v.over8 <= opts.tolerate))
  if (failing.length) {
    console.error(`FAIL: ${failing.map(([n, v]) => `${n} over8=${v.over8}`).join(', ')} (tolerance ${opts.tolerate}); diffs in ${outDir}`)
    process.exit(1)
  }
  const retried = Object.entries(summary).filter(([, v]) => v.retried).map(([n, v]) => `${n} (first over8=${v.retried.first_over8})`)
  if (retried.length) console.log(`note: passed on retry: ${retried.join(', ')}`)
  console.log(`PASS: every image within tolerance ${opts.tolerate}; written to ${outDir}`)
}

main().catch(err => { console.error(err); process.exit(1) })
