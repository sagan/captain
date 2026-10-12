import { fileURLToPath, URL } from 'node:url'
import { readFileSync, readdirSync } from 'node:fs'
import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'
import tailwindcss from '@tailwindcss/vite'

// The React port of the Glassmorphism status page. Mirrors
// vite.glass.config.ts exactly except for the framework plugin and the
// output directory, so the two bundles can be served under the same
// preset switch and diffed against each other.
export default defineConfig({
  root: fileURLToPath(new URL('./glass-react', import.meta.url)),
  base: './',
  plugins: [react(), tailwindcss(), {
    name: 'glass-licenses',
    generateBundle() {
      this.emitFile({ type: 'asset', fileName: 'licenses/theme-MIT.txt', source: readFileSync(new URL('./glass/LICENSE', import.meta.url), 'utf8') })
      for (const name of readdirSync(new URL('./glass/licenses/', import.meta.url))) {
        this.emitFile({ type: 'asset', fileName: `licenses/${name}`, source: readFileSync(new URL(`./glass/licenses/${name}`, import.meta.url), 'utf8') })
      }
    },
  }],
  resolve: { alias: {
    '@': fileURLToPath(new URL('./glass-react/src', import.meta.url)),
    // Framework-neutral code is shared with the Vue theme, not copied.
    '@shared': fileURLToPath(new URL('./glass-shared', import.meta.url)),
  } },
  define: { __BUILD_VERSION__: JSON.stringify('3.3.7'), __BUILD_GIT_HASH__: JSON.stringify('06999d5') },
  css: { postcss: { plugins: [] } },
  server: { port: 5178, proxy: { '/api': 'http://127.0.0.1:8080' } },
  build: { outDir: '../dist/glass-react', emptyOutDir: true },
})
