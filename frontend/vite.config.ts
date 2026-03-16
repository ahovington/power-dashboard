import { defineConfig } from 'vite';
import react from '@vitejs/plugin-react';

export default defineConfig({
  plugins: [react()],
  test: {
    environment: 'jsdom',
    globals: true,
    setupFiles: './src/tests/setup.ts',
    include: ['src/**/*.test.{ts,tsx}'],
    passWithNoTests: true,
  },
  server: {
    proxy: {
      '/api': 'http://localhost:8080',
    },
  },
});
