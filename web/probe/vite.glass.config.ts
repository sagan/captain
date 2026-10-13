import { fileURLToPath, URL } from 'node:url'
import { readFileSync, readdirSync } from 'node:fs'
import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'
import tailwindcss from '@tailwindcss/vite'

// The Glassmorphism status page (React port of the MIT Vue theme). Built into
// dist/glass, which the Go server embeds for the "glass" preset.
export default defineConfig({
  root: fileURLToPath(new URL('./glass', import.meta.url)),
  base: './',
  // Same static files (flags, OS icons, globe textures) as the Vue build.
  publicDir: fileURLToPath(new URL('./glass/public', import.meta.url)),
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
    '@': fileURLToPath(new URL('./glass/src', import.meta.url)),
    // Framework-neutral code is shared with the Vue theme, not copied.
    '@shared': fileURLToPath(new URL('./glass-shared', import.meta.url)),
  } },
  define: { __BUILD_VERSION__: JSON.stringify('3.3.7'), __BUILD_GIT_HASH__: JSON.stringify('06999d5') },
  css: { postcss: { plugins: [] } },
  server: { port: 5178, proxy: { '/api': 'http://127.0.0.1:8080' } },
  build: { outDir: '../dist/glass', emptyOutDir: true },
})
