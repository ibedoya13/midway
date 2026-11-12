import { fileURLToPath, URL } from 'node:url'

import vue from '@vitejs/plugin-vue'
import { defineConfig } from 'vite'

export default defineConfig({
  plugins: [vue()],
  resolve: {
    alias: {
      '@': fileURLToPath(new URL('./src', import.meta.url)),
    },
  },
  server: {
    port: 5173,
    // Escucha en todas las interfaces para poder abrirlo desde el móvil.
    host: true,
  },
  preview: {
    port: 4173,
    host: true,
  },
})
