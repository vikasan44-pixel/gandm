// Settlement currencies offered in price forms. Mirrors the backend allow-list
// in internal/money/currency.go — keep the two in sync when adding a corridor.
// The platform is worldwide: each deal is priced in the currency the party
// naming the price picks (no FX conversion).
export const CURRENCIES = [
  "USD",
  "EUR",
  "CNY",
  "RUB",
  "KZT",
  "GBP",
  "TRY",
  "AED",
  "KRW",
  "JPY",
  "INR",
  "PLN",
  "KGS",
  "UZS",
  "GEL",
  "AZN",
  "BYN",
  "BRL",
  "IRR",
  "PKR",
  "MNT",
] as const;

export type Currency = (typeof CURRENCIES)[number];

// Default pre-selected currency in forms. Matches the backend Fallback.
export const DEFAULT_CURRENCY: Currency = "USD";
