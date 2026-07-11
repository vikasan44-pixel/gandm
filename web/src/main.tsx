import { StrictMode } from "react";
import { createRoot } from "react-dom/client";
import App from "./App";
import "./styles/global.css";

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

