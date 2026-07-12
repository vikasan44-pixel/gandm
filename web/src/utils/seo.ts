import { useEffect } from "react";

interface SeoOptions {
  // Заголовок вкладки/страницы (без суффикса — добавляется « — GANDM»).
  title: string;
  // Описание для поисковой выдачи. Если не задано — остаётся дефолтное из index.html.
  description?: string;
  // true → добавить <meta name="robots" content="noindex"> на эту страницу
  // (вход/регистрация/404 — незачем индексировать).
  noindex?: boolean;
}

// Лёгкое управление <head> для конкретной страницы без внешних зависимостей.
// Всё восстанавливается при уходе со страницы (SPA-навигация), поэтому noindex
// со страницы входа не «протекает» на публичный лендинг.
export function useSeo({ title, description, noindex }: SeoOptions): void {
  useEffect(() => {
    const prevTitle = document.title;
    document.title = `${title} — GANDM`;

    let descEl: HTMLMetaElement | null = null;
    let prevDesc: string | null = null;
    if (description) {
      descEl = document.head.querySelector('meta[name="description"]');
      if (descEl) {
        prevDesc = descEl.getAttribute("content");
        descEl.setAttribute("content", description);
      }
    }

    let robotsEl: HTMLMetaElement | null = null;
    if (noindex) {
      robotsEl = document.createElement("meta");
      robotsEl.setAttribute("name", "robots");
      robotsEl.setAttribute("content", "noindex, follow");
      document.head.appendChild(robotsEl);
    }

    return () => {
      document.title = prevTitle;
      if (descEl && prevDesc !== null) descEl.setAttribute("content", prevDesc);
      if (robotsEl) robotsEl.remove();
    };
  }, [title, description, noindex]);
}
