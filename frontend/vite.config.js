import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

// https://vitejs.dev/config/
export default defineConfig({
  plugins: [react()],
  server: {
    proxy: {
      // Każde zapytanie z frontendu zaczynające się od '/api'
      // zostanie przekierowane na adres 'http://localhost:8080'
      '/api': {
        target: 'http://localhost:8080',
        changeOrigin: true,
      }
    }
  }
})