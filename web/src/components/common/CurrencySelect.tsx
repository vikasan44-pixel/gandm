import { CURRENCIES } from "../../utils/currency";

// CurrencySelect — the settlement currency picker shown next to every price
// input. Worldwide marketplace: the party naming the price chooses. Codes come
// from utils/currency (mirrors the backend allow-list).
export function CurrencySelect({
  value,
  onChange,
  className,
  ariaLabel,
}: {
  value: string;
  onChange: (v: string) => void;
  className?: string;
  ariaLabel?: string;
}) {
  return (
    <select
      className={className ?? "field__input"}
      value={value}
      onChange={(e) => onChange(e.target.value)}
      aria-label={ariaLabel}
    >
      {CURRENCIES.map((c) => (
        <option key={c} value={c}>
          {c}
        </option>
      ))}
    </select>
  );
}
