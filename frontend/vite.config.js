import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import path from 'node:path'

// Build output goes directly into the Go binary embed directory.
// base: platform assets live under /static/ so they never collide with the
// user container's root-relative assets (/assets, /favicon, ...).
export default defineConfig({
  plugins: [vue()],
  base: '/static/',
  build: {
    outDir: path.resolve(__dirname, '../backend/internal/api/web/dist'),
    emptyOutDir: true,
  },
  server: {
    proxy: {
      '/api': 'http://localhost:8080',
      '/static': 'http://localhost:8080',
    },
  },
})
