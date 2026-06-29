import { defineConfig } from 'vite'
import tailwindcss from '@tailwindcss/vite'
import { fileURLToPath, URL } from 'node:url'

export default defineConfig({
  publicDir: false,
  plugins: [tailwindcss()],
  resolve: {
    alias: {
      // Use the framework's official realtime client straight from source.
      '@oniworks/socket': fileURLToPath(new URL('../../client/oni-socket.ts', import.meta.url)),
    },
  },
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
