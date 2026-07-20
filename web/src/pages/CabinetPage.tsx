import { useEffect, useState, type FormEvent } from "react";
import { Link } from "react-router-dom";
import { t } from "../i18n";
import { useAuth } from "../auth/AuthContext";
import { updateMyProfile } from "../api/participant";
import { ApiError } from "../api/client";
import type { LegalForm } from "../api/types";

const cards = [
  { key: "cargo", to: "/app/cargo" },
  { key: "transport", to: "/app/fleet" },
  { key: "warehouse", to: "/app/warehouses" },
  { key: "customs", to: "/app/customs" },
] as const;

export function CabinetPage() {
  const { user, applyUserProfile } = useAuth();
  const [name, setName] = useState(user?.company_name ?? "");
  const [legalForm, setLegalForm] = useState<LegalForm>(user?.legal_form ?? "individual");
  const [saving, setSaving] = useState(false);
  const [message, setMessage] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    setName(user?.company_name ?? "");
    setLegalForm(user?.legal_form ?? "individual");
  }, [user]);

  async function saveProfile(event: FormEvent) {
    event.preventDefault();
    setSaving(true);
    setError(null);
    setMessage(null);
    try {
      const response = await updateMyProfile(name, legalForm);
      applyUserProfile(response.user);
      setMessage(response.user.status === "pending" ? t("cabinet.profileReverification") : t("cabinet.profileSaved"));
    } catch (err) {
      setError(err instanceof ApiError ? err.message : t("common.unexpectedError"));
    } finally {
      setSaving(false);
    }
  }

  return (
    <div className="page cabinet-home">
      <div>
        <h1 className="page__title">{t("cabinet.title")}</h1>
        <p className="marketplace-page__hint">{t("cabinet.hint")}</p>
      </div>

      <section className="panel cabinet-home__profile">
        <h2 className="cabinet-home__section-title">{t("cabinet.profileTitle")}</h2>
        <p className="marketplace-page__hint">{t("cabinet.profileHint")}</p>
        <form className="form-grid" onSubmit={saveProfile}>
          <label className="field">
            <span className="field__label">{t("register.legalForm")}</span>
            <select value={legalForm} onChange={(event) => setLegalForm(event.target.value as LegalForm)}>
              <option value="individual">{t("legalForm.individual")}</option>
              <option value="legal_entity">{t("legalForm.legal_entity")}</option>
            </select>
          </label>
          <label className="field">
            <span className="field__label">{t(legalForm === "individual" ? "register.personName" : "register.companyName")}</span>
            <input value={name} onChange={(event) => setName(event.target.value)} required />
          </label>
          {error && <div className="form-error">{error}</div>}
          {message && <div className="form-success">{message}</div>}
          <button className="btn btn--primary" type="submit" disabled={saving}>{saving ? t("common.loading") : t("common.save")}</button>
        </form>
      </section>

      <section>
        <h2 className="cabinet-home__section-title">{t("cabinet.myListings")}</h2>
        <div className="cabinet-home__grid">
          {cards.map((card) => (
            <Link className="cabinet-home__card" to={card.to} key={card.key}>
              <strong>{t(`cabinet.${card.key}Title`)}</strong>
              <span>{t(`cabinet.${card.key}Text`)}</span>
              <span className="cabinet-home__card-action">{t("cabinet.open")} →</span>
            </Link>
          ))}
        </div>
      </section>

      <section>
        <h2 className="cabinet-home__section-title">{t("cabinet.account")}</h2>
        <div className="cabinet-home__quick-links">
          <Link to="/app/my-tools">{t("myTools.navLabel")}</Link>
          <Link to="/app/chats">{t("nav.chats")}</Link>
          <Link to="/app/notifications">{t("nav.notifications")}</Link>
          <Link to="/app/rating">{t("rating.navLabel")}</Link>
        </div>
      </section>
    </div>
  );
}
