// Vendor the exact icons referenced by the public theme; no visitor CDN requests.
import { readFileSync, writeFileSync, readdirSync } from 'node:fs'
import { createRequire } from 'node:module'
import { resolve } from 'node:path'
const require = createRequire(import.meta.url)
function files(dir) { return readdirSync(dir, { withFileTypes: true }).flatMap(e => e.isDirectory() ? files(`${dir}/${e.name}`) : [`${dir}/${e.name}`]) }
const names = new Set(files('glass/src').filter(p => /\.(vue|ts)$/.test(p)).flatMap(p => [...readFileSync(p,'utf8').matchAll(/['"`]([a-z][a-z0-9-]+:[a-z][a-z0-9-]*)['"`]/g)].map(m => m[1])))
const collections = {}, licenses = {}
for (const full of names) {
 const [prefix, name] = full.split(':')
 let source
 try { source = require(`@iconify/json/json/${prefix}.json`) } catch { continue }
 collections[prefix] ??= { prefix, icons: {}, aliases: {}, width: source.width, height: source.height }
 function add(n) {
   if (source.icons[n]) collections[prefix].icons[n] = source.icons[n]
   else if (source.aliases?.[n]) { collections[prefix].aliases[n] = source.aliases[n]; add(source.aliases[n].parent) }
   else { console.warn(`Missing icon ${prefix}:${n}; using local help icon`); collections[prefix].icons[n] = {body: '<path d="M12 18h.01M9.1 9a3 3 0 0 1 5.8 1c0 2-3 2-3 4" fill="none" stroke="currentColor" stroke-width="2"/>',width:24,height:24} }
 }
 add(name)
 licenses[prefix] = source.info?.license
}
writeFileSync('glass/src/icons.json', JSON.stringify(Object.values(collections)))
writeFileSync('glass/ICON-LICENSES.json', JSON.stringify(licenses,null,2)+'\n')
console.log(`${names.size} referenced icons bundled locally`)
