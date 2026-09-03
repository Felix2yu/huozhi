const fs = require('fs');
let code = fs.readFileSync('node_modules/vite-plugin-pwa/dist/index.js', 'utf8');

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
