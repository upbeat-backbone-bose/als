import js from '@eslint/js'
import vue from 'eslint-plugin-vue'

export default [
  {
    ignores: ['dist/**', 'speedtest/**', 'public/speedtest_worker.js', '.eslintrc.cjs']
  },
  js.configs.recommended,
  ...vue.configs['flat/essential'],
  {
    files: ['**/*.{js,cjs,mjs,jsx,vue}'],
    languageOptions: {
      ecmaVersion: 'latest',
      sourceType: 'module'
    },
    rules: {
      'no-undef': 'off',
      'no-empty': 'off',
      'no-unused-vars': 'off',
      'no-prototype-builtins': 'off',
      'vue/multi-word-component-names': 'off',
      'vue/no-side-effects-in-computed-properties': 'off',
      'vue/require-v-for-key': 'off',
      'vue/valid-v-for': 'off'
    }
  }
]
