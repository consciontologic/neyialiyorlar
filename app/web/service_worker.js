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

// Fetch: cache-first for static assets; API calls pass through untouched.
// We never intercept /api/ requests — the browser handles them directly so
// Firefox (and any browser) uses its own XHR/fetch without SW involvement.
self.addEventListener('fetch', (event) => {
  const { request } = event;
  const url = new URL(request.url);

  // Never intercept non-GET or API requests
  if (request.method !== 'GET' || url.pathname.startsWith('/api/')) {
    return;
  }

  // Static assets: cache-first
  event.respondWith(
    caches.match(request).then((response) => {
      if (response) {
        return response;
      }
      return fetch(request).then((response) => {
        // Only cache immutable assets
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
});
