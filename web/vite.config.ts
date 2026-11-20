import path from 'path'
import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'
import tailwindcss from '@tailwindcss/vite'
import { tanstackRouter } from '@tanstack/router-plugin/vite'

// The dev server proxies to the Go API, so the browser sees a single origin
// (no CORS in development). In production a reverse proxy does the same job.
const api = { target: 'http://localhost:3000', changeOrigin: true }

export default defineConfig({
  plugins: [
    tanstackRouter({ target: 'react', autoCodeSplitting: true }),
    react(),
    tailwindcss(),
  ],
  resolve: {
    alias: {
      '@': path.resolve(__dirname, './src'),
    },
  },
  server: {
    port: 5174,
    allowedHosts: true,
    proxy: {
      '/api': { ...api, rewrite: (p) => p.replace(/^\/api/, '') },
      // pages rendered by the Go app (order status, tracking, invoice)
      '/order': api,
      '/orders': api,
      '/track': api,
    },
  },
})
