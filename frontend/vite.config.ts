import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

// https://vitejs.dev/config/
export default defineConfig({
  plugins: [react()],
  server: {
    port: 3000,
    proxy: {
      // Proxy MCP requests to backend during development
      '/mcp': {
        target: 'http://localhost:8080',
        changeOrigin: true,
      },
      // Proxy SSE events to backend during development
      '/events': {
        target: 'http://localhost:8080',
        changeOrigin: true,
      },
    },
  },
  build: {
    outDir: 'dist',
    emptyOutDir: true,
    chunkSizeWarningLimit: 1000, // Increase from default 500 kB to 1000 kB
  },
})

