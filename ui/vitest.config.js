import { defineConfig } from 'vitest/config'
import { createSharedPlugins, resolveAlias } from './vite.shared.js'

export default defineConfig({
  plugins: createSharedPlugins(),
  resolve: {
    alias: resolveAlias
  },
  test: {
    environment: 'jsdom',
    globals: true,
    setupFiles: ['./src/test-setup.js'],
    exclude: [
      'speedtest/**',
      'node_modules/**',
      'dist/**'
    ]
  }
})
