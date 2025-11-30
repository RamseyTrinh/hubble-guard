import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

// https://vitejs.dev/config/
export default defineConfig({
  plugins: [react()],
  server: {
    port: 5000,
    proxy: {
      '/api': {
        target: 'http://localhost:5001',
        changeOrigin: true,
        secure: false,
      },
      '/ws': {
        target: 'ws://localhost:5001',
        ws: true,
      },
      '/grafana': {
        target: 'http://localhost:3000',
        changeOrigin: true,
        secure: false,
        rewrite: (path) => path.replace(/^\/grafana/, ''),
        configure: (proxy, _options) => {
          proxy.on('proxyReq', (proxyReq, req, _res) => {
            // Remove X-Frame-Options header to allow embedding
            proxyReq.removeHeader('x-frame-options')
          })
          proxy.on('proxyRes', (proxyRes, req, _res) => {
            // Remove X-Frame-Options from response
            delete proxyRes.headers['x-frame-options']
            // Add CORS headers
            proxyRes.headers['access-control-allow-origin'] = '*'
            proxyRes.headers['access-control-allow-methods'] = 'GET, POST, PUT, DELETE, OPTIONS'
          })
        },
      }
    }
  },
  build: {
    outDir: 'dist',
    assetsDir: 'assets',
  }
})

