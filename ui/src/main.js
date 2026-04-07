import './assets/base.css'
import { createApp } from 'vue'
import { createPinia } from 'pinia'
import App from './App.vue'
import { setupI18n } from './config/lang.js'

async function bootstrap() {
  const app = createApp(App)
  const i18n = await setupI18n()

  app.use(i18n)
  app.use(createPinia())
  app.mount('#app')
}

bootstrap()
