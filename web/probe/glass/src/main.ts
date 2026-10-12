import { createPinia } from 'pinia'
import { createApp } from 'vue'
import { setupIconify } from '@/iconify'
import { message } from '@shared/utils/message'
import App from './App.vue'
import router from './router'

import '@shared/styles/main.css'

window.$message = message

setupIconify().catch((err) => {
  console.warn('[main] iconify init failed', err)
})

const pinia = createPinia()
const app = createApp(App)

app.use(pinia)
app.use(router)

app.mount('#app')
