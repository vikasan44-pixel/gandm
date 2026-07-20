import { t } from "../../i18n";

export function CustomsCatalogPage() {
  return (
    <div className="page">
      <h1 className="page__title">{t("customsCatalog.title")}</h1>
      <section className="panel marketplace-empty">
        <h2 className="panel__title">{t("customsCatalog.inProgress")}</h2>
        <p>{t("customsCatalog.description")}</p>
      </section>
    </div>
  );
}
