import { getLocale } from "../i18n";
import { cityNameFromLabel } from "./locationLabel";

// pickLabel выбирает локализованную подпись, а для карточек и списков оставляет
// только город/населённый пункт. Полный адрес продолжает храниться в GeoPoint
// и показывается только внутри поля выбора адреса и карты.
export function pickLabel(
  labels: Record<string, string> | undefined | null,
  fallback: string
): string {
  let selected = fallback;
  if (labels) {
    const loc = getLocale();
    selected = labels[loc] || labels.en || labels.ru || labels.zh || fallback;
  }
  return cityNameFromLabel(selected);
}
