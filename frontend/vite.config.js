import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import path from 'node:path'

// Build output goes directly into the Go binary embed directory.
// base: platform assets live under /dc-static/ (unique, never collides with
// user container asset roots like /static, /assets, /favicon, ...).
export default defineConfig({
  plugins: [vue()],
  base: '/dc-static/',
  build: {
    outDir: path.resolve(__dirname, '../backend/internal/api/web/dist'),
    emptyOutDir: true,
  },
  server: {
    proxy: {
      '/api': 'http://localhost:8080',
      '/dc-static': 'http://localhost:8080',
    },
  },
})
