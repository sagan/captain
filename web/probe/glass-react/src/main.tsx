import { createRoot } from 'react-dom/client'
import { setupIconify } from '@/iconify'
import { message } from '@shared/utils/message'
import App from './App'

import '@shared/styles/main.css'

window.$message = message

try {
  setupIconify()
}
catch (err) {
  console.warn('[main] iconify init failed', err)
}

createRoot(document.getElementById('app')!).render(<App />)
