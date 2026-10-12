import { createRoot } from 'react-dom/client'
import { App } from './App'
import '@shared/styles/main.css'

createRoot(document.getElementById('app')!).render(<App />)
