import { fileURLToPath, URL } from 'node:url'

import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'

// https://vitejs.dev/config/
export default defineConfig({
  plugins: [vue()],
  resolve: {
    alias: {
      '@': fileURLToPath(new URL('./src', import.meta.url))
    }
  },
  build: {
    outDir: "../webui/",
    assetsDir: "secured",
    rollupOptions: {
      output: {
        entryFileNames: `secured/[name].js`,
        chunkFileNames: `secured/[name].js`,
        assetFileNames: `secured/[name].[ext]`
      }
    },
  },
})
