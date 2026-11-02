import { writeFileSync } from 'node:fs';
import { defineConfig, type Plugin } from 'vite';
import { svelte } from '@sveltejs/vite-plugin-svelte';

// go:embed needs dist/ to exist even before the first build, so the
// placeholder file is kept in git and restored after Vite empties the folder.
const keepPlaceholder: Plugin = {
  name: 'keep-gitkeep',
  closeBundle() {
    writeFileSync('../internal/webui/dist/.gitkeep', '');
  },
};

// The Go binary embeds the build output (internal/webui/dist).
// In development, run the app with:
//   go run ./cmd/sabeel-whatsapp-tools --demo --no-browser --port 8765 \
//     --dev-origin http://localhost:5173 --dev-token devtoken123
// then open http://localhost:5173/#token=devtoken123
export default defineConfig({
  plugins: [svelte(), keepPlaceholder],
  base: './',
  build: {
    outDir: '../internal/webui/dist',
    emptyOutDir: true,
    assetsInlineLimit: 0,
    target: 'es2022',
    reportCompressedSize: false,
  },
  server: {
    port: 5173,
    strictPort: true,
    proxy: {
      '/api': { target: 'http://127.0.0.1:8765', changeOrigin: true },
    },
  },
});
