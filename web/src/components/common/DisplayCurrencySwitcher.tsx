import { useRates } from "../../money/RatesContext";
import { CURRENCIES } from "../../utils/currency";
import { t } from "../../i18n";

// DisplayCurrencySwitcher lets the user pick which currency the approximate "≈"
// hints are shown in. Only meaningful once NBK rates have loaded; hidden
// otherwise so it never appears dead.
export function DisplayCurrencySwitcher() {
  const { rates, displayCurrency, setDisplayCurrency } = useRates();
  if (Object.keys(rates).length === 0) return null;

  return (
    <label className="display-currency">
      <span className="display-currency__label">{t("common.displayIn")}</span>
      <select
        className="display-currency__select"
        value={displayCurrency}
        onChange={(e) => setDisplayCurrency(e.target.value)}
        aria-label={t("common.displayIn")}
      >
        {CURRENCIES.map((c) => (
          <option key={c} value={c}>
            {c}
          </option>
        ))}
      </select>
    </label>
  );
}
