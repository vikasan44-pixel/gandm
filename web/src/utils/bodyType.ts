import { t } from "../i18n";

// Тип кузова хранится СТАБИЛЬНЫМ ключом (не переведённой строкой), чтобы
// объявление показывалось на языке интерфейса, а не на языке, на котором его
// подали. Ключи совпадают с i18n-ключами fleet.* Неизвестное значение
// (например, кузов, введённый вручную в старых данных) показываем как есть.
export const BODY_TYPE_KEYS = [
  "bodyTented",
  "bodyFlatbed",
  "bodyLowboy",
  "bodyReefer",
  "bodyContainer",
  "bodyOther",
] as const;

export function bodyTypeLabel(value: string): string {
  return (BODY_TYPE_KEYS as readonly string[]).includes(value) ? t(`fleet.${value}`) : value;
}
