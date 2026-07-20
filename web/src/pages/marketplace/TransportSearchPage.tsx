import { LandingSearch } from "../../components/landing/LandingSearch";
import { t } from "../../i18n";

export function TransportSearchPage() {
  return (
    <div className="page marketplace-page">
      <div>
        <h1 className="page__title">{t("marketplace.transportTitle")}</h1>
        <p className="marketplace-page__hint">{t("marketplace.transportHint")}</p>
      </div>
      <LandingSearch initialTab="transport" showTabs={false} isAuthenticated />
    </div>
  );
}
