import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

export default defineConfig({
  plugins: [react()],
  server: {
    port: 3000,
    strictPort: false,
    proxy: {
      '/diagnose': {
        target: 'http://localhost:8080',
        changeOrigin: true,
      },
      '/diagnose/history': {
        target: 'http://localhost:8080',
        changeOrigin: true,
      },
    }
  },
  build: {
    outDir: 'dist',
    sourcemap: true,
  }
})
