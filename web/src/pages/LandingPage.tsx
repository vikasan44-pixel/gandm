import { Link, Navigate } from "react-router-dom";
import { cabinetPathFor, useAuth } from "../auth/AuthContext";
import { LocaleSwitcher } from "../components/common/LocaleSwitcher";
import { LandingSearch } from "../components/landing/LandingSearch";
import { t } from "../i18n";

// Гостевая страница: рекламирует, что человек может делать на платформе, и
// ведёт на регистрацию/вход. Залогиненных сразу уводит в их кабинет.
export function LandingPage() {
  const { kind, user, isReady } = useAuth();
  if (!isReady) return null;
  if (kind === "admin") return <Navigate to="/admin/dashboard" replace />;
  if (kind === "user" && user) return <Navigate to={cabinetPathFor(user)} replace />;

  return (
    <div className="landing">
      <LocaleSwitcher />
      <header className="landing__hero">
        <h1 className="landing__title">{t("landing.heroTitle")}</h1>
        <p className="landing__subtitle">{t("landing.heroSubtitle")}</p>
        <div className="landing__cta">
          <Link className="btn btn--primary landing__cta-btn" to="/register">
            {t("landing.ctaRegister")}
          </Link>
          <Link className="btn btn--secondary landing__cta-btn" to="/login">
            {t("landing.ctaLogin")}
          </Link>
        </div>
      </header>

      <LandingSearch />

      <section className="landing__columns">
        <div className="landing__card">
          <h2 className="landing__card-title">{t("landing.forClientsTitle")}</h2>
          <ul className="landing__list">
            <li>{t("landing.forClients1")}</li>
            <li>{t("landing.forClients2")}</li>
            <li>{t("landing.forClients3")}</li>
            <li>{t("landing.forClients4")}</li>
          </ul>
        </div>
        <div className="landing__card">
          <h2 className="landing__card-title">{t("landing.forPartnersTitle")}</h2>
          <ul className="landing__list">
            <li>{t("landing.forPartners1")}</li>
            <li>{t("landing.forPartners2")}</li>
            <li>{t("landing.forPartners3")}</li>
            <li>{t("landing.forPartners4")}</li>
          </ul>
        </div>
      </section>

      <section className="landing__how">
        <h2 className="landing__how-title">{t("landing.howTitle")}</h2>
        <ol className="landing__steps">
          {(["1", "2", "3", "4"] as const).map((n) => (
            <li key={n} className="landing__step">
              <span className="landing__step-num">{n}</span>
              <div>
                <div className="landing__step-title">{t(`landing.how${n}Title`)}</div>
                <div className="landing__step-text">{t(`landing.how${n}`)}</div>
              </div>
            </li>
          ))}
        </ol>
      </section>

      <footer className="landing__trust">{t("landing.trustNote")}</footer>
    </div>
  );
}
