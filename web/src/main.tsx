import { StrictMode } from "react";
import { createRoot } from "react-dom/client";
import App from "./App";
import "./styles/global.css";

// A production preview may previously have registered the PWA worker on the
// same localhost origin. Merely not registering it in dev is not enough: an
// existing worker keeps controlling Vite and can serve stale JS/CSS forever.
// Remove old workers and their GANDM caches before mounting the dev app.
if (import.meta.env.DEV && "serviceWorker" in navigator) {
  void navigator.serviceWorker.getRegistrations().then(async (registrations) => {
    if (registrations.length === 0) return;
    await Promise.all(registrations.map((registration) => registration.unregister()));
    if ("caches" in window) {
      const keys = await caches.keys();
      await Promise.all(keys.filter((key) => key.startsWith("gandm-")).map((key) => caches.delete(key)));
    }
    // One controlled response may already have come from the old cache. A
    // single automatic reload hands the page back to Vite after unregister.
    if (sessionStorage.getItem("gandm-dev-sw-cleaned") !== "1") {
      sessionStorage.setItem("gandm-dev-sw-cleaned", "1");
      window.location.reload();
    }
  });
}

const rootElement = document.getElementById("root");
if (!rootElement) {
  throw new Error("#root element not found");
}

createRoot(rootElement).render(
  <StrictMode>
    <App />
  </StrictMode>
);

// PWA: регистрируем service worker только в production-сборке — в dev он
// мешает Vite HMR и кэширует модули. В превью (vite dev) SW не активен;
// устанавливаемость проверяется на собранном билде (npm run build && preview).
if (import.meta.env.PROD && "serviceWorker" in navigator) {
  window.addEventListener("load", () => {
    navigator.serviceWorker.register("/sw.js").catch(() => {
      /* регистрация не критична — приложение работает и без SW */
    });
  });
}
