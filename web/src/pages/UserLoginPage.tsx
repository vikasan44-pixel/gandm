import { useState, type FormEvent } from "react";
import { Link, Navigate, useNavigate } from "react-router-dom";
import { cabinetPathFor, useAuth } from "../auth/AuthContext";
import { ApiError } from "../api/client";
import { LocaleSwitcher } from "../components/common/LocaleSwitcher";
import { t } from "../i18n";
import { useSeo } from "../utils/seo";

export function UserLoginPage() {
  const { loginUser, kind, user } = useAuth();
  const navigate = useNavigate();
  useSeo({ title: t("login.userTitle"), noindex: true });
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [isSubmitting, setIsSubmitting] = useState(false);

  if (kind === "user" && user) {
    return <Navigate to={cabinetPathFor(user)} replace />;
  }

  async function handleSubmit(e: FormEvent) {
    e.preventDefault();
    setError(null);
    setIsSubmitting(true);
    try {
      const loggedIn = await loginUser(email, password);
      navigate(cabinetPathFor(loggedIn), { replace: true });
    } catch (err) {
      setError(err instanceof ApiError ? err.message : t("common.unexpectedError"));
    } finally {
      setIsSubmitting(false);
    }
  }

  return (
    <div className="login-screen">
      <form className="login-card" onSubmit={handleSubmit}>
        <h1 className="login-card__title">{t("login.userTitle")}</h1>
        <label className="field">
          <span className="field__label">{t("login.email")}</span>
          <input
            type="email"
            value={email}
            onChange={(e) => setEmail(e.target.value)}
            autoFocus
            required
          />
        </label>
        <label className="field">
          <span className="field__label">{t("login.password")}</span>
          <input
            type="password"
            value={password}
            onChange={(e) => setPassword(e.target.value)}
            required
          />
        </label>
        {error && <div className="form-error">{error}</div>}
        <button className="btn btn--primary" type="submit" disabled={isSubmitting}>
          {isSubmitting ? t("common.loading") : t("login.submit")}
        </button>
        <Link className="login-card__switch" to="/register">
          {t("login.toRegister")}
        </Link>
        <Link className="login-card__switch" to="/admin/login">
          {t("login.toAdminLogin")}
        </Link>
        <LocaleSwitcher />
      </form>
    </div>
  );
}
