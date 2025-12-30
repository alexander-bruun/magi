const CACHE_NAME = 'magi-v1';
const urlsToCache = [
  '/',
  '/assets/css/vendor/core.min.css',
  '/assets/css/vendor/utilities.min.css',
  '/assets/css/styles.css',
  '/assets/js/vendor/core.iife.js',
  '/assets/js/vendor/icon.iife.js',
  '/assets/js/vendor/htmx.min.js',
  '/assets/js/vendor/htmx-ext-ws.js',
  '/assets/js/vendor/htmx-ext-form-json.js',
  '/assets/js/magi.js',
  '/assets/js/reader.js',
  '/assets/manifest.json'
];

// Install event - cache resources
self.addEventListener('install', event => {
  event.waitUntil(
    caches.open(CACHE_NAME)
      .then(cache => {
        return cache.addAll(urlsToCache);
      })
  );
});

// Fetch event - serve from cache when offline
self.addEventListener('fetch', event => {
  event.respondWith(
    caches.match(event.request)
      .then(response => {
        // Return cached version or fetch from network
        return response || fetch(event.request);
      })
  );
});

// Activate event - clean up old caches
self.addEventListener('activate', event => {
  event.waitUntil(
    caches.keys().then(cacheNames => {
      return Promise.all(
        cacheNames.map(cacheName => {
          if (cacheName !== CACHE_NAME) {
            return caches.delete(cacheName);
          }
        })
      );
    })
  );
});