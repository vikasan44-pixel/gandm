import type { CargoCategory, GeoPoint } from "../../api/types";
import { getLocale } from "../../i18n";
import { cargoCategoryLabel, otherLocales } from "../../utils/cargoCategories";
import { cityLabel } from "../../utils/locationLabel";

export function MultilingualRoute({ origin, destination }: { origin: GeoPoint; destination: GeoPoint }) {
  const current = getLocale();
  return (
    <span className="multilingual-label">
      <span>{cityLabel(origin, current)} → {cityLabel(destination, current)}</span>
      <span className="multilingual-label__translations">
        {otherLocales().map((locale) => `${cityLabel(origin, locale)} → ${cityLabel(destination, locale)}`).join(" · ")}
      </span>
    </span>
  );
}

export function MultilingualCargoCategory({ category }: { category: CargoCategory }) {
  return (
    <span className="multilingual-label">
      <span>{cargoCategoryLabel(category)}</span>
      <span className="multilingual-label__translations">
        {otherLocales().map((locale) => cargoCategoryLabel(category, locale)).join(" · ")}
      </span>
    </span>
  );
}
