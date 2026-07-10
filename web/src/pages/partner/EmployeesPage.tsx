import { useState, type FormEvent } from "react";
import { useAsync } from "../../hooks/useAsync";
import {
  createEmployee,
  getEmployees,
  setEmployeeBlocked,
} from "../../api/participant";
import { LoadingState } from "../../components/common/LoadingState";
import { ErrorState } from "../../components/common/ErrorState";
import { ApiError } from "../../api/client";
import { t } from "../../i18n";
import { formatDateTime } from "../../utils/date";

// EmployeesPage — суб-аккаунты сотрудников (ТЗ §13.1). Доступ гейтится
// бэкендом по инструменту manage_employees; без него список отвечает 403.
export function EmployeesPage() {
  const employees = useAsync(getEmployees, []);
  const [email, setEmail] = useState("");
  const [phone, setPhone] = useState("");
  const [password, setPassword] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [notice, setNotice] = useState<string | null>(null);
  const [isSubmitting, setIsSubmitting] = useState(false);

  async function handleCreate(e: FormEvent) {
    e.preventDefault();
    setError(null);
    setNotice(null);
    setIsSubmitting(true);
    try {
      await createEmployee(email, phone, password);
      setNotice(t("employees.added"));
      setEmail("");
      setPhone("");
      setPassword("");
      employees.reload();
    } catch (err) {
      setError(err instanceof ApiError ? err.message : t("common.unexpectedError"));
    } finally {
      setIsSubmitting(false);
    }
  }

  async function handleToggle(id: string, blocked: boolean) {
    setError(null);
    try {
      await setEmployeeBlocked(id, blocked);
      employees.reload();
    } catch (err) {
      setError(err instanceof ApiError ? err.message : t("common.unexpectedError"));
    }
  }

  return (
    <div className="page">
      <h1 className="page__title">{t("employees.title")}</h1>
      <p className="panel__hint">{t("employees.hint")}</p>

      <section className="panel">
        <h2 className="panel__title">{t("employees.addTitle")}</h2>
        <form className="inline-form" onSubmit={handleCreate}>
          <input
            type="email"
            value={email}
            onChange={(e) => setEmail(e.target.value)}
            placeholder={t("employees.email")}
            required
          />
          <input
            type="tel"
            value={phone}
            onChange={(e) => setPhone(e.target.value)}
            placeholder={t("employees.phone")}
          />
          <input
            type="password"
            value={password}
            onChange={(e) => setPassword(e.target.value)}
            placeholder={t("employees.password")}
            minLength={8}
            required
          />
          <button className="btn btn--primary btn--sm" type="submit" disabled={isSubmitting}>
            {isSubmitting ? t("common.loading") : t("employees.add")}
          </button>
        </form>
        {notice && <p className="panel__hint">{notice}</p>}
        {error && <div className="form-error">{error}</div>}
      </section>

      <section className="panel">
        {employees.isLoading && <LoadingState />}
        {employees.error && <ErrorState message={employees.error} onRetry={employees.reload} />}
        {employees.data && employees.data.length === 0 && (
          <p className="panel__hint">{t("employees.empty")}</p>
        )}
        {employees.data && employees.data.length > 0 && (
          <ul className="tool-group__list">
            {employees.data.map((emp) => (
              <li key={emp.id} className="tool-row">
                <div>
                  <div className="tool-row__name">
                    {emp.email}{" "}
                    <span className={emp.status === "active" ? "pill pill--green" : "pill pill--red"}>
                      {t(`status.user.${emp.status}`)}
                    </span>
                  </div>
                  <div className="tool-row__key">
                    {emp.phone || "—"} · {formatDateTime(emp.created_at)}
                  </div>
                </div>
                <button
                  className="btn btn--ghost btn--sm"
                  onClick={() => void handleToggle(emp.id, emp.status === "active")}
                >
                  {emp.status === "active" ? t("employees.deactivate") : t("employees.activate")}
                </button>
              </li>
            ))}
          </ul>
        )}
      </section>
    </div>
  );
}
