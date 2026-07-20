import { createContext, useContext, useEffect, useState, type ReactNode } from "react";
import { getCurrencyRates } from "../api/rates";
import { DEFAULT_CURRENCY } from "../utils/currency";

const DISPLAY_KEY = "gandm_display_ccy";

interface RatesState {
  rates: Record<string, number>; // KZT per 1 unit; empty until loaded
  date: string;
  displayCurrency: string;
  setDisplayCurrency: (c: string) => void;
}

const RatesContext = createContext<RatesState | null>(null);

function initialDisplay(): string {
  const stored = localStorage.getItem(DISPLAY_KEY);
  return stored || DEFAULT_CURRENCY;
}

// RatesProvider fetches the NBK snapshot once and holds the user's chosen
// display currency (persisted in localStorage). Rates are public reference
// data; a failed fetch just leaves rates empty and no hint is shown.
export function RatesProvider({ children }: { children: ReactNode }) {
  const [rates, setRates] = useState<Record<string, number>>({});
  const [date, setDate] = useState("");
  const [displayCurrency, setDisplayCurrencyState] = useState<string>(initialDisplay);

  useEffect(() => {
    let cancelled = false;
    getCurrencyRates()
      .then((r) => {
        if (!cancelled) {
          setRates(r.rates ?? {});
          setDate(r.date ?? "");
        }
      })
      .catch(() => {
        // No rates → components simply render amounts without the "≈" hint.
      });
    return () => {
      cancelled = true;
    };
  }, []);

  function setDisplayCurrency(c: string) {
    localStorage.setItem(DISPLAY_KEY, c);
    setDisplayCurrencyState(c);
  }

  return (
    <RatesContext.Provider value={{ rates, date, displayCurrency, setDisplayCurrency }}>
      {children}
    </RatesContext.Provider>
  );
}

export function useRates(): RatesState {
  const ctx = useContext(RatesContext);
  if (!ctx) {
    // Safe fallback if used outside the provider (e.g. isolated tests).
    return { rates: {}, date: "", displayCurrency: DEFAULT_CURRENCY, setDisplayCurrency: () => {} };
  }
  return ctx;
}

// approxConvert converts an amount from one currency to another via KZT using
// the NBK rates. Returns null when either currency is unknown or equal (no hint
// needed).
export function approxConvert(
  amount: number,
  from: string,
  to: string,
  rates: Record<string, number>
): number | null {
  if (from === to) return null;
  const rFrom = rates[from];
  const rTo = rates[to];
  if (!rFrom || !rTo) return null;
  return (amount * rFrom) / rTo;
}
