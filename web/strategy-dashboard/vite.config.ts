import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'
import tailwindcss from '@tailwindcss/vite'

export default defineConfig({
  plugins: [react(), tailwindcss()],
  base: '/dashboard/',
  server: {
    proxy: {
      '/health': 'http://127.0.0.1:9020',
      '/instances': 'http://127.0.0.1:9020',
      '/report': 'http://127.0.0.1:9020',
      '/instruments': 'http://127.0.0.1:9020',
      '/assistant': 'http://127.0.0.1:9020',
      '/config': 'http://127.0.0.1:9020',
      '/ai-chat': 'http://127.0.0.1:9020',
      '/ai-trader': 'http://127.0.0.1:9020',
      '/advisor': 'http://127.0.0.1:9030',
    },
  },
  build: {
    outDir: 'dist',
    emptyOutDir: true,
  },
})
