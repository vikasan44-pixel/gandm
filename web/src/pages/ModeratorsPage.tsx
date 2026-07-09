import { useState, type FormEvent } from "react";
import { useAsync } from "../hooks/useAsync";
import { createModerator, getModerators } from "../api/admin";
import { LoadingState } from "../components/common/LoadingState";
import { ErrorState } from "../components/common/ErrorState";
import { ApiError } from "../api/client";
import { t } from "../i18n";
import { formatDateTime } from "../utils/date";

// ModeratorsPage (ТЗ §19.6): список staff-аккаунтов и создание модераторов.
// Полных админов через UI не создать — только модераторов с ограниченными
// правами (верификация + просмотр участников), enforcement на бэкенде.
export function ModeratorsPage() {
  const moderators = useAsync(getModerators, []);
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [notice, setNotice] = useState<string | null>(null);
  const [isSubmitting, setIsSubmitting] = useState(false);

  async function handleAdd(e: FormEvent) {
    e.preventDefault();
    setError(null);
    setNotice(null);
    setIsSubmitting(true);
    try {
      await createModerator(email, password);
      setNotice(t("moderators.added"));
      setEmail("");
      setPassword("");
      moderators.reload();
    } catch (err) {
      setError(err instanceof ApiError ? err.message : t("common.unexpectedError"));
    } finally {
      setIsSubmitting(false);
    }
  }

  return (
    <div className="page">
      <h1 className="page__title">{t("moderators.title")}</h1>
      <p className="panel__hint">{t("moderators.hint")}</p>

      <section className="panel">
        <h2 className="panel__title">{t("moderators.addTitle")}</h2>
        <form className="inline-form" onSubmit={handleAdd}>
          <input
            type="email"
            value={email}
            onChange={(e) => setEmail(e.target.value)}
            placeholder={t("moderators.email")}
            required
          />
          <input
            type="password"
            value={password}
            onChange={(e) => setPassword(e.target.value)}
            placeholder={t("moderators.password")}
            minLength={8}
            required
          />
          <button className="btn btn--primary btn--sm" type="submit" disabled={isSubmitting}>
            {isSubmitting ? t("common.loading") : t("moderators.add")}
          </button>
        </form>
        {notice && <p className="panel__hint">{notice}</p>}
        {error && <div className="form-error">{error}</div>}
      </section>

      <section className="panel">
        {moderators.isLoading && <LoadingState />}
        {moderators.error && <ErrorState message={moderators.error} onRetry={moderators.reload} />}
        {moderators.data && (
          <ul className="tool-group__list">
            {moderators.data.map((m) => (
              <li key={m.id} className="tool-row">
                <div>
                  <div className="tool-row__name">{m.email}</div>
                  <div className="tool-row__key">{formatDateTime(m.created_at)}</div>
                </div>
                <span className={m.role === "admin" ? "pill pill--red" : "pill pill--neutral"}>
                  {m.role === "admin" ? t("moderators.roleAdmin") : t("moderators.roleModerator")}
                </span>
              </li>
            ))}
          </ul>
        )}
      </section>
    </div>
  );
}
