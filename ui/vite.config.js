// vite.config.ts
import { fileURLToPath, URL } from 'node:url'
import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import AutoImport from 'unplugin-auto-import/vite'
import Components from 'unplugin-vue-components/vite'
import { NaiveUiResolver } from 'unplugin-vue-components/resolvers'
import fs from 'node:fs'
import path from 'node:path'

// https://vitejs.dev/config/
export default defineConfig(({ command }) => {
  return {
    base: './',
    server: {
      proxy: {
        '/session': {
          target: 'http://127.0.0.1:8080',
          ws: true
        },
        '/method': {
          target: 'http://127.0.0.1:8080',
          ws: true
        }
      }
    },
    resolve: {
      alias: {
        '@': fileURLToPath(new URL('./src', import.meta.url))
      }
    },
    plugins: [
      vue(),
      AutoImport({
        imports: [
          'vue',
          {
            'naive-ui': ['useDialog', 'useMessage', 'useNotification', 'useLoadingBar']
          }
        ]
      }),
      {
        name: 'build-script',
        closeBundle() {
          if (command !== 'build') {
            return
          }

          const source = path.join(__dirname, 'speedtest', 'speedtest_worker.js')
          const dest = path.join(__dirname, 'dist', 'speedtest_worker.js')

          if (!fs.existsSync(source)) {
            throw new Error(`[build-script] Missing source file: ${source}`)
          }

          fs.copyFileSync(source, dest)
        }
      },
      Components({
        resolvers: [NaiveUiResolver()]
      })
    ]
  }
})
