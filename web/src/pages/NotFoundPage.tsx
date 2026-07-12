import { Link } from "react-router-dom";
import { t } from "../i18n";
import { useSeo } from "../utils/seo";

// Реальная страница «не найдено» для гостей на неизвестном пути — вместо
// молчаливого редиректа на «/» (тот выглядел как soft-404 для поисковика).
// noindex: незачем индексировать несуществующие адреса.
export function NotFoundPage() {
  useSeo({ title: t("notFound.title"), noindex: true });
  return (
    <main className="login-screen">
      <div className="login-card" style={{ textAlign: "center" }}>
        <h1 className="login-card__title">{t("notFound.title")}</h1>
        <p className="register-hint">{t("notFound.text")}</p>
        <Link className="btn btn--primary" to="/">
          {t("notFound.back")}
        </Link>
      </div>
    </main>
  );
}
