import { api } from "./client";

// Official NBK exchange-rate snapshot: `rates[code]` = KZT for one unit of that
// currency (KZT itself is 1). Used only for the approximate "≈" display hint —
// deal amounts are never converted.
export interface CurrencyRates {
  base: string;
  date: string;
  rates: Record<string, number>;
}

export function getCurrencyRates() {
  return api.get<CurrencyRates>("/currency-rates");
}
