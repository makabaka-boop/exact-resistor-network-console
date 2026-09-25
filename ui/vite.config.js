import { defineConfig } from 'vite';
import { svelte } from '@sveltejs/vite-plugin-svelte';

export default defineConfig({
  plugins: [svelte()],
  server: {
    port: 5173,
    proxy: {
      // dev server proxies the solver API to a locally running circuit service
      '/api': 'http://localhost:8080'
    }
  }
});
