import { defineConfig } from 'vite';
import react from '@vitejs/plugin-react';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { VitePWA } from 'vite-plugin-pwa';

const __dirname = path.dirname(fileURLToPath(import.meta.url));

// https://vitejs.dev/config/
export default defineConfig({
  plugins: [
    react(),
    VitePWA({
      registerType: 'autoUpdate',
      includeAssets: ['favicon.svg', 'robots.txt'],
      // 生成 SW 时的 Workbox 运行时缓存规则
      workbox: {
        runtimeCaching: [
          // API GET 请求：NetworkFirst（在线拿最新，断网回退缓存快照）
          {
            urlPattern: /^\/api\//,
            handler: 'NetworkFirst',
            method: 'GET',
            options: {
              cacheName: 'hz-api-cache',
              expiration: {
                maxEntries: 500,
                maxAgeSeconds: 60 * 60 * 24 * 7, // 7 天
              },
              cacheableResponse: { statuses: [0, 200] },
            },
          },
          // 上传的图片/头像：CacheFirst（一次请求永久受益）
          {
            urlPattern: /^\/uploads\//,
            handler: 'CacheFirst',
            options: {
              cacheName: 'hz-uploads-cache',
              expiration: {
                maxEntries: 200,
                maxAgeSeconds: 60 * 60 * 24 * 30, // 30 天
              },
            },
          },
        ],
      },
      manifest: {
        name: '货殖',
        short_name: '货殖',
        description: '一个简洁纯粹的个人记账系统',
        theme_color: '#10B981',
        background_color: '#ffffff',
        display: 'standalone',
        start_url: '/',
        icons: [
          {
            src: '/icon-192.svg',
            sizes: '192x192',
            type: 'image/svg+xml',
            purpose: 'any maskable',
          },
          {
            src: '/icon-512.svg',
            sizes: '512x512',
            type: 'image/svg+xml',
            purpose: 'any maskable',
          },
        ],
      },
    }),
  ],
  resolve: {
    alias: {
      '@': path.resolve(__dirname, './src'),
    },
  },
  server: {
    port: 5173,
    host: '0.0.0.0',
    proxy: {
      '/api': {
        target: 'http://localhost:8080',
        changeOrigin: true,
      },
      '/uploads': {
        target: 'http://localhost:8080',
        changeOrigin: true,
      },
    },
  },
  build: {
    outDir: 'dist',
    chunkSizeWarningLimit: 1500,
    rollupOptions: {
      output: {
        manualChunks(id) {
          if (id.includes('node_modules')) {
            if (id.includes('react') || id.includes('react-router')) return 'react-vendor';
            if (id.includes('recharts')) return 'chart-vendor';
            if (id.includes('lucide') || id.includes('sonner')) return 'ui-vendor';
          }
        },
      },
    },
  },
});
