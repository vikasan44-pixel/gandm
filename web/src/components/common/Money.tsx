import { useRates, approxConvert } from "../../money/RatesContext";
import { t } from "../../i18n";

// Money renders a settlement amount in its own currency, and — when NBK rates
// are loaded and the currency differs from the user's chosen display currency
// — appends an approximate "≈ N DISPLAY" hint. The hint is display-only; the
// real amount and currency never change.
export function Money({ amount, currency }: { amount: number; currency: string }) {
  const { rates, date, displayCurrency } = useRates();
  const converted = approxConvert(amount, currency, displayCurrency, rates);

  return (
    <span className="money">
      {amount.toLocaleString()} {currency}
      {converted != null && (
        <span
          className="money__approx"
          title={`${t("common.nbkRate")}${date ? ` (${date})` : ""}`}
        >
          {" "}
          ≈ {converted.toLocaleString(undefined, { maximumFractionDigits: 0 })} {displayCurrency}
        </span>
      )}
    </span>
  );
}
