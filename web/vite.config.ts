import path from 'node:path'
import tailwindcss from '@tailwindcss/vite'
import react from '@vitejs/plugin-react'
import { defineConfig, loadEnv } from 'vite'

function requireEnv(env: Record<string, string>, name: string): string {
  const value = env[name]?.trim()
  if (!value) {
    throw new Error(`${name} is required to run the dev server (see web/.env.example)`)
  }
  return value
}

export default defineConfig(({ command, mode }) => {
  const plugins = [react(), tailwindcss()]
  const resolve = { alias: { '@': path.resolve(import.meta.dirname, './src') } }
  if (command !== 'serve' || mode === 'test') {
    return {
      plugins,
      resolve,
      test: {
        environment: 'jsdom',
        setupFiles: ['./src/test/setup.ts'],
        restoreMocks: true,
        coverage: {
          provider: 'v8' as const,
          include: ['src/**/domain/**/*.ts', 'src/**/datasource/**/*.ts'],
          exclude: ['src/**/*.test.ts'],
          reporter: ['text-summary', 'html'],
        },
      },
    }
  }

  const env = loadEnv(mode, process.cwd(), '')
  const apiTarget = requireEnv(env, 'DEV_API_PROXY_TARGET')
  const port = Number(requireEnv(env, 'DEV_SERVER_PORT'))
  return {
    plugins,
    resolve,
    server: {
      port,
      strictPort: true,
      proxy: {
        '/api': { target: apiTarget, changeOrigin: false },
        '/.well-known': { target: apiTarget, changeOrigin: false },
      },
    },
  }
})
