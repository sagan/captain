import assert from 'node:assert/strict'
import test from 'node:test'
import { forwardTargetAddress, forwardTargetProtocol } from '../src/lib/ingress.ts'
import { forwardHealth } from '../src/lib/forward-health.ts'

test('IPv6 managed targets use brackets for direct, line and NAT mappings', () => {
 const base = {ib:{Port:1081},node:{domain:'',public_addr:'2001:db8::10'}}
 assert.equal(forwardTargetAddress(base),'[2001:db8::10]:1081')
 const ingress = {line_ip:'2001:db8::20',entry_host:'2001:db8::30',entry_domain:'',port_offset:2000}
 assert.equal(forwardTargetAddress({...base,ingress}),'[2001:db8::20]:1081')
 assert.equal(forwardTargetAddress({...base,ingress:{...ingress,line_ip:''}}),'[2001:db8::30]:3081')
 assert.equal(forwardTargetAddress({...base,ingress:{...ingress,line_ip:'',port_mappings:[{local_from:1080,local_to:1089,public_from:5000}]}}),'[2001:db8::30]:5001')
 assert.equal(forwardTargetAddress({...base,node:{domain:'example.com',public_addr:'2001:db8::10'}}),'example.com:1081')
})
test('UDP protocols select UDP forwarding, manual selection can override afterwards', () => {
 for(const p of ['hysteria2','tuic']) assert.equal(forwardTargetProtocol(p),'udp')
 assert.equal(forwardTargetProtocol('vless'),'both')
})
test('legacy UDP reports never appear as healthy or offline based on TCP', () => {
 for(const up of [true,false]) for(const protocol of ['udp','both']) {
  const h=forwardHealth({up,rtt_ms:0},protocol)
  assert.equal(h.state,'unknown'); assert.equal(h.color,'gray')
 }
 const h=forwardHealth({up:false,rtt_ms:0,health:'down',probe_protocol:'tcp'},'both')
 assert.equal(h.udpUnknown,true);assert.equal(h.transport,'tcp')
 const failed=forwardHealth({up:false,rtt_ms:0,health:'down',probe_protocol:'backend'},'udp')
 assert.equal(failed.color,'red');assert.equal(failed.transport,'backend')
})
