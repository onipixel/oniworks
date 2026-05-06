import { defineConfig } from 'vite'
import tailwindcss from '@tailwindcss/vite'

export default defineConfig({
  publicDir: false,
  plugins: [tailwindcss()],
  build: {
    outDir: 'public/build',
    emptyOutDir: true,
    manifest: true,
    rollupOptions: {
      input: { app: 'resources/ts/app.ts' },
    },
  },
  server: {
    port: 5173,
    proxy: {
      '/api': 'http://localhost:8003',
      '/storage': 'http://localhost:8003',
      '/ws': { target: 'ws://localhost:8003', ws: true },
    },
  },
})
