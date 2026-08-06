import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'
import path from 'path'

export default defineConfig({
  plugins: [react()],
  base: '/admin/dashboard/',
  build: {
    // The Go binary embeds this directory directly. Keep one generated copy;
    // web/dist is intentionally retired and ignored.
    outDir: '../internal/static/dist',
    emptyOutDir: true,
  },
  resolve: {
    alias: {
      '@': path.resolve(__dirname, './src'),
    },
  },
  server: {
    port: 5173,
    proxy: {
      '/admin/api': 'http://localhost:8081',
      '/metrics': 'http://localhost:8081',
    },
  },
})
