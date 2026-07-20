import { getLocale, type Locale } from "../i18n";

type LocationLabelPoint = {
  label: string;
  labels?: Record<string, string> | null;
};

const BROAD_ADMIN_PART = /(?:область|район|округ|край|республик|провинц|region|district|county|province|prefecture|autonomous|自治区|自治州|省|区)$/i;
const STREET_PART = /(?:улица|проспект|шоссе|переулок|набережная|street|road|avenue|lane|highway|路|街)/i;
const POSTCODE_PART = /^(?=.*\d)[A-ZА-ЯЁ0-9][A-ZА-ЯЁ0-9 -]{2,11}$/i;
const HOUSE_NUMBER_PART = /^\d+[A-ZА-ЯЁ]?(?:[/-]\d+[A-ZА-ЯЁ]?)?$/i;
const COUNTRY_PART = /^(?:казахстан|қазақстан|kazakhstan|中国|китай|china|россия|russia|кыргызстан|kyrgyzstan|узбекистан|uzbekistan)$/i;
const CITY_ADMIN_PART = /^(.+?)\s+(?:г\.?\s*а\.?|городская администрация|city administration)$/i;

const KNOWN_CITIES: Array<{ names: Record<Locale, string>; aliases: string[] }> = [
  { names: { ru: "Алматы", en: "Almaty", zh: "阿拉木图" }, aliases: ["алматы", "алматинск", "almaty", "阿拉木图"] },
  { names: { ru: "Караганда", en: "Karaganda", zh: "卡拉干达" }, aliases: ["караганда", "карагандинск", "karaganda", "卡拉干达"] },
  { names: { ru: "Астана", en: "Astana", zh: "阿斯塔纳" }, aliases: ["астана", "нур-султан", "nur-sultan", "astana", "阿斯塔纳"] },
  { names: { ru: "Актобе", en: "Aktobe", zh: "阿克托别" }, aliases: ["актобе", "актюб", "aktobe", "阿克托别"] },
  { names: { ru: "Шымкент", en: "Shymkent", zh: "奇姆肯特" }, aliases: ["шымкент", "чимкент", "shymkent", "奇姆肯特"] },
  { names: { ru: "Урумчи", en: "Urumqi", zh: "乌鲁木齐" }, aliases: ["урумчи", "urumqi", "wulumuqi", "乌鲁木齐"] },
  { names: { ru: "Хоргос", en: "Khorgos", zh: "霍尔果斯" }, aliases: ["хоргос", "khorgos", "horgos", "霍尔果斯"] },
  { names: { ru: "Кашгар", en: "Kashgar", zh: "喀什" }, aliases: ["кашгар", "kashgar", "kashi", "喀什"] },
  { names: { ru: "Талдыкорган", en: "Taldykorgan", zh: "塔尔迪库尔干" }, aliases: ["талдыкорган", "taldykorgan", "塔尔迪库尔干"] },
];

function extractCity(label: string): string {
  const parts = label.split(",").map((part) => part.trim()).filter(Boolean);
  for (let index = parts.length - 1; index >= 0; index -= 1) {
    const part = parts[index];
    if (COUNTRY_PART.test(part) || POSTCODE_PART.test(part) || HOUSE_NUMBER_PART.test(part) || STREET_PART.test(part) || BROAD_ADMIN_PART.test(part)) continue;
    const namedCity = part.match(/^(?:акимат\s+)?города?\s+(.+)$/i);
    if (namedCity) return namedCity[1].trim();
    const administration = part.match(CITY_ADMIN_PART);
    if (administration) return administration[1].replace(/инская$/i, "а").replace(/ская$/i, "");
    return part;
  }
  return parts[0] || label;
}

function knownCityFromLabels(labels: string[], locale: Locale): string | null {
  const parts = labels.flatMap((label) => label.split(",").map((part) => part.trim().toLocaleLowerCase()));
  const known = KNOWN_CITIES.find((city) => city.aliases.some((alias) => parts.includes(alias)));
  return known?.names[locale] ?? null;
}

export function cityLabel(point: LocationLabelPoint, locale: Locale = getLocale()): string {
  const localized = point.labels?.[locale] || point.labels?.en || point.labels?.ru || point.labels?.zh || point.label;
  const known = knownCityFromLabels([point.label, ...Object.values(point.labels ?? {})], locale);
  return known ?? extractCity(localized);
}

export function cityNameFromLabel(label: string, locale: Locale = getLocale()): string {
  return cityLabel({ label }, locale);
}

export function compactDirectionLabel(label: string, locale: Locale = getLocale()): string {
  const points = label.split(/\s*→\s*/).filter(Boolean);
  if (points.length < 2) return cityNameFromLabel(label, locale);
  return points.map((point) => cityNameFromLabel(point, locale)).join(" → ");
}
