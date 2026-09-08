import { defineConfig, loadEnv } from 'vite'
import vue from '@vitejs/plugin-vue'

function parseHost(raw: string): string | boolean {
  const v = raw.trim().toLowerCase()
  if (!v || v === 'true' || v === '0.0.0.0') return true
  if (v === 'false') return '127.0.0.1'
  return raw.trim()
}

function parseAllowedHosts(raw: string, lan: boolean): true | string[] | undefined {
  const v = raw.trim()
  if (!v) return lan ? true : undefined
  const lower = v.toLowerCase()
  if (lower === 'true' || v === '*') return true
  if (lower === 'false') return undefined
  return v.split(',').map((s) => s.trim()).filter(Boolean)
}

export default defineConfig(({ mode }) => {
  const env = loadEnv(mode, process.cwd(), '')
  const host = parseHost(env.VITE_DEV_HOST || '0.0.0.0')
  const port = Number(env.VITE_DEV_PORT || 5173)
  const proxyTarget = (env.VITE_API_PROXY || 'http://127.0.0.1:9808').replace(/\/$/, '')
  const lan = host === true || (typeof host === 'string' && host !== '127.0.0.1' && host !== 'localhost')

  return {
    plugins: [vue()],
    server: {
      host,
      port: Number.isFinite(port) && port > 0 ? port : 5173,
      allowedHosts: parseAllowedHosts(env.VITE_ALLOWED_HOSTS || '', lan),
      proxy: {
        '/api': {
          target: proxyTarget,
          changeOrigin: true,
        },
      },
    },
  }
})
