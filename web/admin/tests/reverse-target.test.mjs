import assert from 'node:assert/strict'
import test from 'node:test'
import { withReverseProtocol, withReverseTarget } from '../src/lib/reverse-target.ts'

const connection = () => ({
  transit_id: 7, server_name: 'old.example.com',
  settings: { flow: 'xtls-rprx-vision', tls: { mode: 2, server_name: 'old.example.com', reality: { handshake_server: 'old.example.com', handshake_port: 443, private_key: 'fixture', short_ids: ['0123'] } } },
})

test('main target selection and advanced selection round-trip into the same save payload', () => {
  const original = connection()
  const first = withReverseTarget(original, 'first.example.com')
  assert.equal(first.settings.tls.server_name, 'first.example.com')
  assert.equal(first.settings.tls.reality.handshake_server, 'first.example.com')
  const edited = structuredClone(first.settings)
  edited.tls.server_name = 'second.example.com'
  edited.tls.reality.handshake_server = 'second.example.com'
  const saved = JSON.parse(JSON.stringify(withReverseProtocol(first, edited)))
  assert.equal(saved.server_name, 'second.example.com')
  assert.equal(saved.settings.tls.server_name, saved.server_name)
  assert.equal(saved.settings.tls.reality.handshake_server, saved.server_name)
  assert.equal(saved.settings.tls.reality.private_key, 'fixture')
  assert.deepEqual(saved.settings.tls.reality.short_ids, ['0123'])
  assert.equal(saved.settings.flow, 'xtls-rprx-vision')
  assert.deepEqual(original, connection())
})

test('bulk target changes preserve ordinary TLS settings and an independent tunnel target', () => {
  const ordinary = { server_name: 'tunnel.example.com', settings: { tls: { mode: 1, server_name: 'user.example.com', auto_cert: true } } }
  const changed = withReverseTarget(ordinary, 'new-tunnel.example.com')
  assert.equal(changed.server_name, 'new-tunnel.example.com')
  assert.deepEqual(changed.settings, ordinary.settings)
  assert.equal(withReverseProtocol(changed, { tls: { mode: 1, server_name: 'cert.example.com' } }).server_name, 'new-tunnel.example.com')
})

test('switching protocols and clearing a target never restores a stale SNI', () => {
  const original = connection()
  const cleared = withReverseTarget(original, '')
  assert.equal(cleared.settings.tls.reality.handshake_server, '')
  const advanced = structuredClone(original.settings)
  advanced.tls.server_name = '   '
  assert.equal(withReverseProtocol(original, advanced).server_name, '')
  const newDraft = withReverseTarget({ server_name: '' }, 'new.example.com')
  assert.equal(newDraft.server_name, 'new.example.com')
  assert.equal(newDraft.settings, undefined)
})
