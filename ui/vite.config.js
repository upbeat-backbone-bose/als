import { defineConfig } from 'vite'
import { createSharedPlugins, resolveAlias } from './vite.shared.js'

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
      alias: resolveAlias
    },
    plugins: createSharedPlugins({ command })
  }
})
