import assert from 'node:assert/strict'
import test from 'node:test'
import { setImmediate } from 'node:timers/promises'
import { runNodeJob } from '../src/lib/api.ts'

test('scans queue and poll only the selected transit', async t => {
  t.mock.timers.enable({ apis: ['setTimeout'] })
  const calls = []
  const results = [{ host: 'target.example.com', feasible: true, tls_record_ok: true }]
  t.mock.method(globalThis, 'fetch', async (url, init) => {
    calls.push({ url, init })
    return new Response(JSON.stringify(init.method === 'POST' ? { id: 'job-1' } : { done_at: '2026-10-06T00:00:00Z', result: results }))
  })
  const controller = new AbortController()
  const running = runNodeJob(42, 'reality_scan', { hosts: ['target.example.com'] }, 150000, controller.signal)
  await setImmediate()
  t.mock.timers.tick(2000)
  assert.deepEqual(await running, results)
  assert.deepEqual(calls.map(c => c.url), ['/api/admin/nodes/42/jobs', '/api/admin/nodes/42/jobs/job-1'])
  assert.deepEqual(JSON.parse(calls[0].init.body), { kind: 'reality_scan', params: { hosts: ['target.example.com'] } })
  assert.equal(calls[1].init.signal, controller.signal)
})

test('closing a scan while waiting cancels polling', async t => {
  const calls = []
  t.mock.method(globalThis, 'fetch', async (url) => { calls.push(url); return new Response(JSON.stringify({ id: 'job-2' })) })
  const controller = new AbortController()
  const running = runNodeJob(43, 'reality_scan', { hosts: [] }, 150000, controller.signal)
  const rejected = assert.rejects(running, { name: 'AbortError' })
  await setImmediate()
  controller.abort()
  await rejected
  assert.deepEqual(calls, ['/api/admin/nodes/43/jobs'])
})

test('node job errors are reported instead of returning an empty success', async t => {
  t.mock.timers.enable({ apis: ['setTimeout'] })
  t.mock.method(globalThis, 'fetch', async (_url, init) => new Response(JSON.stringify(init.method === 'POST' ? { id: 'job-3' } : { done_at: '2026-10-06T00:00:00Z', error: 'scan failed' })))
  const running = runNodeJob(44, 'reality_scan', { hosts: [] })
  const rejected = assert.rejects(running, /scan failed/)
  await setImmediate()
  t.mock.timers.tick(2000)
  await rejected
})
