import ru from "./ru";
import en from "./en";
import zh from "./zh";

// Русский, китайский, английский (ТЗ §14). en/zh типизированы как
// Dictionary (typeof ru) — tsc не даст словарям разъехаться по ключам.
const dictionaries = { ru, en, zh };
export type Locale = keyof typeof dictionaries;

export const LOCALES: { code: Locale; label: string }[] = [
  { code: "ru", label: "Рус" },
  { code: "zh", label: "中文" },
  { code: "en", label: "EN" },
];

const STORAGE_KEY = "gandm_locale";

export function getLocale(): Locale {
  const stored = localStorage.getItem(STORAGE_KEY);
  if (stored === "ru" || stored === "en" || stored === "zh") return stored;
  return "ru";
}

// Смена языка перезагружает страницу: t() вызывается при рендере из сотен
// мест без подписки на контекст — reload надёжнее, чем тащить локаль через
// React-контекст во все компоненты.
export function setLocale(locale: Locale) {
  localStorage.setItem(STORAGE_KEY, locale);
  window.location.reload();
}

export function t(path: string): string {
  const parts = path.split(".");
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  let node: any = dictionaries[getLocale()];
  for (const part of parts) {
    node = node?.[part];
  }
  return typeof node === "string" ? node : path;
}
