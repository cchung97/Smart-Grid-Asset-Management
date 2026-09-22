import { readFileSync } from 'node:fs'
import tailwindcss from '@tailwindcss/vite'
import react from '@vitejs/plugin-react'
import { loadEnv } from 'vite'
import { defineConfig } from 'vitest/config'

// https://vite.dev/config/
export default defineConfig(({ mode }) => {
  // API_KEY comes from the repo's .env. Like nginx in Docker, the dev proxy adds it
  // to every /api request, so the browser never sees it.
  const apiKey = loadEnv(mode, '..', '').API_KEY

  // The footer shows package.json's version, so it is bumped in one place.
  const { version } = JSON.parse(readFileSync(new URL('./package.json', import.meta.url), 'utf8')) as { version: string }

  return {
    define: { __APP_VERSION__: JSON.stringify(version) },
    plugins: [react(), tailwindcss()],
    build: {
      // The app has a /assets route; keep the bundle out of a folder of the same name.
      assetsDir: 'static',
    },
    server: {
      proxy: {
        '/api': {
          target: 'http://localhost:8080',
          configure: (proxy) => {
            proxy.on('proxyReq', (req) => {
              if (apiKey) req.setHeader('x-api-key', apiKey)
            })
          },
        },
        '/healthz': 'http://localhost:8080',
        '/swagger': 'http://localhost:8080',
      },
    },
    test: {
      environment: 'jsdom',
      setupFiles: ['./src/test/setup.ts'],
      css: false,
    },
  }
})
