import type { CargoCategory } from "../api/types";
import { getLocale, translate, type Locale } from "../i18n";

export const CARGO_CATEGORIES: CargoCategory[] = [
  "chemicals",
  "equipment",
  "building_materials",
  "home_appliances",
  "furniture",
  "food",
  "textiles",
  "auto_parts",
  "metals",
  "timber",
  "medical_goods",
  "agricultural_goods",
  "plastics",
  "dangerous_goods",
  "other",
];

export function cargoCategoryLabel(category: CargoCategory, locale = getLocale()): string {
  return translate(locale, `cargoCategories.${category}`);
}

export function otherLocales(): Locale[] {
  const current = getLocale();
  return (["ru", "en", "zh"] as Locale[]).filter((locale) => locale !== current);
}
