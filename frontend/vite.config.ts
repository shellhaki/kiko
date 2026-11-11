import react from '@vitejs/plugin-react'
import tailwindcss from '@tailwindcss/vite'
import { defineConfig } from 'vitest/config'

export default defineConfig({
  plugins: [react(), tailwindcss()],
  define: { 'process.env.IS_PREACT': JSON.stringify('false') },
  test: { environment: 'jsdom', server: { deps: { inline: [/@excalidraw/, /open-color/] } } },
})
