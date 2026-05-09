import { defineConfig } from 'vite';
import react from '@vitejs/plugin-react';

// https://vitejs.dev/config/
export default defineConfig({
  plugins: [react()],
  // Use relative paths for VS Code webview compatibility
  base: './',
  server: {
    port: 5173,
    strictPort: true,
  },
  build: {
    outDir: 'dist',
    sourcemap: true,
    rollupOptions: {
      output: {
        // Use consistent filenames (no hash) for easier reference
        entryFileNames: 'assets/[name].js',
        chunkFileNames: 'assets/[name].js',
        assetFileNames: 'assets/[name].[ext]',
        // Code splitting: separate vendor chunks to reduce main bundle size
        manualChunks: {
          // React core
          'vendor-react': ['react', 'react-dom'],
          // Graph visualization (largest dependency)
          'vendor-cytoscape': ['cytoscape', 'cytoscape-dagre', 'dagre'],
          // Code editor
          'vendor-ace': ['ace-builds', 'react-ace'],
          // HTTP client
          'vendor-axios': ['axios'],
        },
      },
    },
  },
});

