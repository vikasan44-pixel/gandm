import { useState, type FormEvent } from "react";
import { Link, Navigate, useNavigate } from "react-router-dom";
import { useAuth } from "../auth/AuthContext";
import { ApiError } from "../api/client";
import { t } from "../i18n";

export function LoginPage() {
  const { loginAdmin, kind } = useAuth();
  const navigate = useNavigate();
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [isSubmitting, setIsSubmitting] = useState(false);

  if (kind === "admin") {
    return <Navigate to="/admin/dashboard" replace />;
  }

  async function handleSubmit(e: FormEvent) {
    e.preventDefault();
    setError(null);
    setIsSubmitting(true);
    try {
      await loginAdmin(email, password);
      navigate("/admin/dashboard", { replace: true });
    } catch (err) {
      setError(err instanceof ApiError ? err.message : t("common.unexpectedError"));
    } finally {
      setIsSubmitting(false);
    }
  }

  return (
    <div className="login-screen">
      <form className="login-card" onSubmit={handleSubmit}>
        <h1 className="login-card__title">{t("login.title")}</h1>
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
        <Link className="login-card__switch" to="/login">
          {t("login.toUserLogin")}
        </Link>
      </form>
    </div>
  );
}
