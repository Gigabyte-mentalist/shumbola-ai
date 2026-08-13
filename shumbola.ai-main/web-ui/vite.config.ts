import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

// https://vite.dev/config/
export default defineConfig({
  plugins: [react()],
  server: {
    // Bind 0.0.0.0 (not just localhost) so operators on the same local
    // network can reach the dev server via this machine's LAN IP.
    host: true,
  },
})
