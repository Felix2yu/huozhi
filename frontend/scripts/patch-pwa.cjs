const fs = require('fs');
let code = fs.readFileSync('node_modules/vite-plugin-pwa/dist/index.js', 'utf8');

// First, restore from npm cache to be safe
const { execSync } = require('child_process');
const contentPath = '/Users/yufei/.npm/_cacache/content-v2/sha512/73/99/0c80df884eb3ad1d7a7c3c0b64dae38811e6ba5e33ffba7086c4e59607343aa9aeb9aa3fdac07834c1ff9017d13fdaef6f87ce8bab13f0cc43cd6ccdabe4';
execSync('rm -rf /tmp/vite-pwa-restore && mkdir -p /tmp/vite-pwa-restore && tar -xzf ' + contentPath + ' -C /tmp/vite-pwa-restore', { encoding: 'utf8' });
code = fs.readFileSync('/tmp/vite-pwa-restore/package/dist/index.js', 'utf8');
console.log('Restored original from npm cache');

// Now apply runtimeCaching patch with proper escaping
// We want this exact code in the file:
//   if (strategies === 'generateSW' && !workbox.runtimeCaching) {
//     workbox.runtimeCaching = [
//       { urlPattern: /^\/api\//, handler: 'NetworkFirst', method: 'GET', ... },
//       { urlPattern: /^\/uploads\//, handler: 'CacheFirst', ... },
//     ];
//   }

// Use Buffer to avoid string escaping issues
const mergeLine = 'const workbox = Object.assign({}, defaultWorkbox, options.workbox || {});';

// Build the injection carefully
const injection = mergeLine + `
  if (strategies === 'generateSW' && !workbox.runtimeCaching) {
    workbox.runtimeCaching = [
      { urlPattern: /^\\/api\\//, handler: 'NetworkFirst', method: 'GET', options: { cacheName: 'hz-api-cache', expiration: { maxEntries: 500, maxAgeSeconds: 604800 }, cacheableResponse: { statuses: [0, 200] } } },
      { urlPattern: /^\\/uploads\\//, handler: 'CacheFirst', options: { cacheName: 'hz-uploads-cache', expiration: { maxEntries: 200, maxAgeSeconds: 2592000 } } },
    ];
  }`;

// Verify the regex will be correct in the output
const regexTest = injection.match(/urlPattern: (\/\^.*?\/)/g);
console.log('Regex patterns in injection:', regexTest);

code = code.replace(mergeLine, injection);
fs.writeFileSync('node_modules/vite-plugin-pwa/dist/index.js', code);

// Final verify
const verifyCode = fs.readFileSync('node_modules/vite-plugin-pwa/dist/index.js', 'utf8');
console.log('Contains hz-api-cache:', verifyCode.includes('hz-api-cache'));
console.log('Contains hz-uploads-cache:', verifyCode.includes('hz-uploads-cache'));

// Check syntax
try {
  new Function(verifyCode.replace(/^import.*$/gm, '').replace(/^export.*$/gm, ''));
  console.log('Syntax check: OK (basic)');
} catch(e) {
  console.log('Syntax warning:', e.message.substring(0, 100));
}
