// Service worker for neyialiyorlar PWA
// Caches app shell + immutable assets; offline shows last-good values with stale flag

const CACHE_NAME = 'neyialiyorlar-v1';
const STATIC_ASSETS = [
  '/',
  '/index.html',
  '/main.dart.js',
  '/manifest.json',
  '/assets/fonts/RobotoMono-Regular.ttf',
  '/assets/fonts/RobotoMono-Bold.ttf',
  '/icons/icon-192x192.png',
  '/icons/icon-512x512.png',
];

// Install: cache app shell
self.addEventListener('install', (event) => {
  event.waitUntil(
    caches.open(CACHE_NAME).then((cache) => {
      console.log('[ServiceWorker] Caching app shell');
      return cache.addAll(STATIC_ASSETS);
    })
  );
  self.skipWaiting();
});

// Activate: clean old caches
self.addEventListener('activate', (event) => {
  event.waitUntil(
    caches.keys().then((cacheNames) => {
      return Promise.all(
        cacheNames.map((cacheName) => {
          if (cacheName !== CACHE_NAME) {
            console.log('[ServiceWorker] Deleting old cache:', cacheName);
            return caches.delete(cacheName);
          }
        })
      );
    })
  );
  self.clients.claim();
});

// Fetch: cache-first for static, network-first for API
self.addEventListener('fetch', (event) => {
  const { request } = event;
  const url = new URL(request.url);

  // Skip non-GET
  if (request.method !== 'GET') {
    return;
  }

  // API calls: network-first, fallback to offline stub
  if (url.pathname.startsWith('/api/')) {
    event.respondWith(
      fetch(request)
        .then((response) => {
          // Don't cache error responses
          if (response.status !== 200) {
            return response;
          }
          // Cache successful API responses
          const clone = response.clone();
          caches.open(CACHE_NAME).then((cache) => {
            cache.put(request, clone);
          });
          return response;
        })
        .catch(() => {
          // Offline: return cached response or offline stub
          return caches.match(request).then((response) => {
            if (response) {
              return response;
            }
            // For API calls, return a stale indicator
            return new Response(
              JSON.stringify({
                error: 'offline',
                message: 'Last-known value shown with stale flag',
              }),
              { status: 200, headers: { 'Content-Type': 'application/json' } }
            );
          });
        })
    );
  } else {
    // Static assets: cache-first
    event.respondWith(
      caches.match(request).then((response) => {
        if (response) {
          return response;
        }
        return fetch(request).then((response) => {
          // Don't cache non-200 or non-immutable
          if (response.status !== 200 || !request.url.includes('/assets/')) {
            return response;
          }
          const clone = response.clone();
          caches.open(CACHE_NAME).then((cache) => {
            cache.put(request, clone);
          });
          return response;
        });
      })
    );
  }
});
