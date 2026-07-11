import { getLocale } from "../i18n";

// pickLabel выбирает подпись точки на языке интерфейса. Fallback: текущий
// язык → английский → русский → китайский → исходная подпись. Так адрес
// читается носителем любого языка, а если перевода нет — показываем как есть.
export function pickLabel(
  labels: Record<string, string> | undefined | null,
  fallback: string
): string {
  if (labels) {
    const loc = getLocale();
    return labels[loc] || labels.en || labels.ru || labels.zh || fallback;
  }
  return fallback;
}
