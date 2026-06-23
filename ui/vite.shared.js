import { fileURLToPath, URL } from 'node:url'
import fs from 'node:fs'
import path from 'node:path'
import vue from '@vitejs/plugin-vue'
import tailwindcss from '@tailwindcss/vite'
import AutoImport from 'unplugin-auto-import/vite'
import Components from 'unplugin-vue-components/vite'
import { NaiveUiResolver } from 'unplugin-vue-components/resolvers'

const projectRoot = fileURLToPath(new URL('.', import.meta.url))

export const resolveAlias = {
  '@': fileURLToPath(new URL('./src', import.meta.url))
}

export function createSharedPlugins({ command } = {}) {
  const plugins = [
    vue(),
    tailwindcss(),
    AutoImport({
      imports: [
        'vue',
        {
          'naive-ui': ['useDialog', 'useMessage', 'useNotification', 'useLoadingBar']
        }
      ]
    })
  ]

  if (command) {
    plugins.push({
      name: 'build-script',
      closeBundle() {
        if (command !== 'build') {
          return
        }

        const source = path.join(projectRoot, 'speedtest', 'speedtest_worker.js')
        const dest = path.join(projectRoot, 'dist', 'speedtest_worker.js')

        if (!fs.existsSync(source)) {
          throw new Error(`[build-script] Missing source file: ${source}`)
        }

        fs.copyFileSync(source, dest)
      }
    })
  }

  plugins.push(
    Components({
      resolvers: [NaiveUiResolver()]
    })
  )

  return plugins
}