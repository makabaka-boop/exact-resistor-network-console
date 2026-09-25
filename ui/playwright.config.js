import { defineConfig } from '@playwright/test';

// The suite runs against a live stack: `docker compose up --build`
// (or `npm run dev` in ui/ with the circuit service on :8080).
export default defineConfig({
  testDir: './tests',
  retries: 0,
  workers: 1,
  reporter: 'list',
  use: {
    baseURL: process.env.BASE_URL || 'http://localhost:5173'
  }
});
