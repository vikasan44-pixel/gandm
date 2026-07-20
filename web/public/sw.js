// GANDM service worker — минимальный и безопасный.
// Цель: устанавливаемость PWA + оболочка приложения работает офлайн.
// Принципы:
//  • /api/* НИКОГДА не кэшируется (это авторизованные данные) — всегда сеть;
//  • навигации: network-first с откатом на закэшированную оболочку (офлайн);
//  • статика того же origin: stale-while-revalidate (быстро + само обновляется);
//  • при активации подчищаем старые версии кэша.
const CACHE = "gandm-shell-v3";
const SHELL = ["/", "/en", "/zh", "/favicon.svg", "/manifest.webmanifest", "/icons/icon-192.png"];

self.addEventListener("install", (event) => {
  event.waitUntil(
    caches.open(CACHE).then((c) => c.addAll(SHELL)).then(() => self.skipWaiting())
  );
});

self.addEventListener("activate", (event) => {
  event.waitUntil(
    caches
      .keys()
      .then((keys) => Promise.all(keys.filter((k) => k !== CACHE).map((k) => caches.delete(k))))
      .then(() => self.clients.claim())
  );
});

self.addEventListener("fetch", (event) => {
  const { request } = event;
  const url = new URL(request.url);

  // A worker left behind by a production preview must never cache Vite dev
  // modules. This also makes local recovery immediate after the worker file
  // updates, instead of stale-while-revalidate returning an old interface.
  if (url.hostname === "127.0.0.1" || url.hostname === "localhost") {
    return;
  }

  // Только GET того же origin проходит через кэш; всё остальное (POST, /api,
  // сторонние тайлы карт/геокодер) — напрямую в сеть.
  if (request.method !== "GET" || url.origin !== self.location.origin || url.pathname.startsWith("/api/")) {
    return;
  }

  // Навигации: свежая версия из сети, при офлайне — закэшированная оболочка.
  if (request.mode === "navigate") {
	const localeShell = url.pathname === "/en" || url.pathname.startsWith("/en/")
	  ? "/en"
	  : url.pathname === "/zh" || url.pathname.startsWith("/zh/") ? "/zh" : "/";
    event.respondWith(
	  fetch(request).catch(() => caches.match(localeShell, { ignoreSearch: true }).then((r) => r || caches.match("/")))
    );
    return;
  }

  // Статика: отдать из кэша сразу, параллельно обновить фоново.
  event.respondWith(
    caches.match(request).then((cached) => {
      const network = fetch(request)
        .then((res) => {
          if (res && res.status === 200 && res.type === "basic") {
            const copy = res.clone();
            caches.open(CACHE).then((c) => c.put(request, copy));
          }
          return res;
        })
        .catch(() => cached);
      return cached || network;
    })
  );
});
