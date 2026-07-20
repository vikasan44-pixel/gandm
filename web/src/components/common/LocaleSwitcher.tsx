import { getLocale, setLocale, LOCALES } from "../../i18n";

// Переключатель языка (ТЗ §14). Смена локали перезагружает страницу —
// см. комментарий в i18n/index.ts.
export function LocaleSwitcher() {
  const current = getLocale();
  return (
    <div className="locale-switcher">
      {LOCALES.map((l) => (
        <button
          type="button"
          key={l.code}
          className={
            "locale-switcher__btn" + (l.code === current ? " locale-switcher__btn--active" : "")
          }
          onClick={() => l.code !== current && setLocale(l.code)}
        >
          {l.label}
        </button>
      ))}
    </div>
  );
}
