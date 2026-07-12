import { defineConfig, loadEnv } from 'vite'
import vue from '@vitejs/plugin-vue'
import { resolve } from 'path'

export default defineConfig(({ mode }) => {
  const env = loadEnv(mode, process.cwd(), '')
  const apiProxyTarget = env.VITE_API_PROXY_TARGET || 'http://localhost:8081'

  return {
    plugins: [vue()],
    resolve: {
      alias: {
        '@': resolve(__dirname, 'src'),
      },
    },
    server: {
      port: 3000,
      proxy: {
        // 注意 key 带 trailing slash：'/api/' 只匹配 /api/<...>，不会误吞
        // 前端 SPA 路径（例：/api-docs、/api-explorer）。否则任何以 /api 开头的
        // URL 都会被 http-proxy 当成 prefix 匹配转给后端。
        '/api/': {
          target: apiProxyTarget,
          changeOrigin: true,
        },
        '/ping': {
          target: apiProxyTarget,
          changeOrigin: true,
        },
        '/swagger': {
          target: apiProxyTarget,
          changeOrigin: true,
        },
        '/openapi.yaml': {
          target: apiProxyTarget,
          changeOrigin: true,
        },
      },
    },
    build: {
      rollupOptions: {
        output: {
          manualChunks(id) {
            if (!id.includes('node_modules')) return undefined
            if (id.includes('/vue/') || id.includes('/vue-router/') || id.includes('/pinia/')) {
              return 'vue'
            }
            if (id.includes('/axios/')) {
              return 'axios'
            }
            return undefined
          },
        },
      },
    },
  }
})
