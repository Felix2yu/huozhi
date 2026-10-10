// 在 vite build 之后再生成最终产物的 Service Worker。
//
// 为什么不能交给 vite.config.ts 里的 vite-plugin-pwa 直接生成：
//   1. 时序：adapter-static 收尾时会 `rm -rf build` 再重写，插件在 client/ssr 构建
//      阶段生成的 sw.js 会被这次清空带走——构建日志里能看到 `files generated
//      build/sw.js` 之后紧跟 `Wrote site to "build"`。
//   2. 内容：插件生成时 build/ 还没有内容，预缓存清单是空的，
//      表现为 `precache 9 entries (0.00 KiB)`，离线能力等于没有。
// 放在 vite build 之后跑，build/ 已就绪，precache 才有真实条目。
//
// 产物落在 build/sw.js，随前端产物一起被复制到后端 embed 目录。
// 后端对 /sw.js 固定下发 no-store（见 backend/internal/router/static.go），
// 保证 autoUpdate 每次都能拉到新版本。
import { generateSW } from 'workbox-build';
import { existsSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
import { dirname, join } from 'node:path';

const here = dirname(fileURLToPath(import.meta.url));
const root = join(here, '..');
const buildDir = join(root, 'build');

if (!existsSync(buildDir)) {
	console.error('[build-sw] 找不到 build/，请先执行 vite build');
	process.exit(1);
}

const { count, size, warnings } = await generateSW({
	globDirectory: buildDir,
	globPatterns: ['**/*.{js,css,html,json,webmanifest,svg,png,ico,woff2}'],
	// sw.js 与 workbox 运行时不能被自己预缓存，否则版本升级时会自锁
	globIgnores: ['sw.js', 'workbox-*.js'],
	swDest: join(buildDir, 'sw.js'),
	cacheId: 'huozhi',
	cleanupOutdatedCaches: true,
	clientsClaim: true,
	// SPA：离线导航回退到 index.html，交给前端路由接管；API 与附件不回退
	navigateFallback: '/index.html',
	navigateFallbackDenylist: [/^\/api\//, /^\/uploads\//],
	mode: 'production',
	runtimeCaching: [
		// API 请求：NetworkFirst（在线优先，离线回退缓存）
		{
			urlPattern: /^\/api\/(?!uploads)/,
			handler: 'NetworkFirst',
			options: {
				cacheName: 'hz-api-cache',
				expiration: { maxEntries: 500, maxAgeSeconds: 60 * 60 * 24 * 7 },
				cacheableResponse: { statuses: [0, 200] }
			}
		},
		// 账单图片：CacheFirst（内容不变，优先读缓存）
		{
			urlPattern: /^\/api\/uploads\//,
			handler: 'CacheFirst',
			options: {
				cacheName: 'hz-uploads-cache',
				expiration: { maxEntries: 200, maxAgeSeconds: 60 * 60 * 24 * 30 }
			}
		},
		// SvelteKit immutable 资源：CacheFirst（文件名带内容哈希）
		{
			urlPattern: /\/_app\/immutable\//,
			handler: 'CacheFirst',
			options: {
				cacheName: 'hz-app-cache',
				expiration: { maxEntries: 200, maxAgeSeconds: 60 * 60 * 24 * 365 }
			}
		}
	]
});

console.log(
	`[build-sw] 已生成 build/sw.js：预缓存 ${count} 个文件 / ${(size / 1024).toFixed(1)} KiB`
);
if (count === 0) {
	console.error('[build-sw] 预缓存清单为空，Service Worker 形同虚设——请检查 globDirectory');
	process.exit(1);
}
for (const w of warnings) console.warn('[build-sw] 警告: ' + w);
