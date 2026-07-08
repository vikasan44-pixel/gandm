import { useState, type FormEvent } from "react";
import { useAsync } from "../../hooks/useAsync";
import { createFillReport, getMyFillReports } from "../../api/participant";
import { LoadingState } from "../../components/common/LoadingState";
import { ErrorState } from "../../components/common/ErrorState";
import { EmptyState } from "../../components/common/EmptyState";
import { ApiError } from "../../api/client";
import { formatDate } from "../../utils/date";
import { t } from "../../i18n";

function todayISO(): string {
  return new Date().toISOString().slice(0, 10);
}

// Warehouse fill reports: expected/actual load + a dated photo as proof.
// Gated server-side by the submit_fill_report tool — a participant without
// it sees a clear 403 message here, not a broken screen.
export function FillReportsPage() {
  const reports = useAsync(getMyFillReports, []);
  const [expected, setExpected] = useState("");
  const [actual, setActual] = useState("");
  const [reportDate, setReportDate] = useState(todayISO());
  const [photo, setPhoto] = useState<File | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [savedNote, setSavedNote] = useState(false);
  const [isSubmitting, setIsSubmitting] = useState(false);
  // Remounts the file input after submit (file inputs can't be cleared via value).
  const [formEpoch, setFormEpoch] = useState(0);

  async function handleSubmit(e: FormEvent) {
    e.preventDefault();
    setError(null);
    setSavedNote(false);

    const expectedNum = Number(expected);
    const actualNum = Number(actual);
    if (
      !Number.isFinite(expectedNum) || expectedNum < 0 || expectedNum > 100 ||
      !Number.isFinite(actualNum) || actualNum < 0 || actualNum > 100
    ) {
      setError(t("fill.percentsRange"));
      return;
    }
    if (!reportDate) {
      setError(t("fill.dateRequired"));
      return;
    }

    setIsSubmitting(true);
    try {
      await createFillReport({
        expectedFillPercent: expectedNum,
        actualFillPercent: actualNum,
        reportDate,
        photo,
      });
      setSavedNote(true);
      setExpected("");
      setActual("");
      setPhoto(null);
      setFormEpoch((v) => v + 1);
      reports.reload();
    } catch (err) {
      setError(err instanceof ApiError ? err.message : t("common.unexpectedError"));
    } finally {
      setIsSubmitting(false);
    }
  }

  return (
    <div className="page">
      <h1 className="page__title">{t("fill.title")}</h1>

      <section className="panel">
        <h2 className="panel__title">{t("fill.formTitle")}</h2>
        <form className="inline-form inline-form--stacked" onSubmit={handleSubmit}>
          <label className="field">
            <span className="field__label">{t("fill.expected")}</span>
            <input
              type="number"
              min="0"
              max="100"
              step="1"
              value={expected}
              onChange={(e) => setExpected(e.target.value)}
            />
          </label>
          <label className="field">
            <span className="field__label">{t("fill.actual")}</span>
            <input
              type="number"
              min="0"
              max="100"
              step="1"
              value={actual}
              onChange={(e) => setActual(e.target.value)}
            />
          </label>
          <label className="field">
            <span className="field__label">{t("fill.date")}</span>
            <input
              type="date"
              value={reportDate}
              onChange={(e) => setReportDate(e.target.value)}
            />
          </label>
          <label className="field">
            <span className="field__label">{t("fill.photo")}</span>
            <input
              key={formEpoch}
              type="file"
              accept="image/jpeg,image/png"
              onChange={(e) => setPhoto(e.target.files?.[0] ?? null)}
            />
          </label>
          {error && <div className="form-error">{error}</div>}
          {savedNote && <div className="detail-panel__resolved">{t("fill.submitted")}</div>}
          <button className="btn btn--primary btn--sm" type="submit" disabled={isSubmitting}>
            {isSubmitting ? t("common.loading") : t("fill.submit")}
          </button>
        </form>
      </section>

      <section className="panel">
        <h2 className="panel__title">{t("fill.historyTitle")}</h2>
        {reports.isLoading && <LoadingState />}
        {reports.error && <ErrorState message={reports.error} onRetry={reports.reload} />}
        {reports.data && reports.data.length === 0 && (
          <EmptyState message={t("fill.historyEmpty")} />
        )}
        {reports.data && reports.data.length > 0 && (
          <ul className="tool-group__list">
            {reports.data.map((report) => (
              <li key={report.id} className="tool-row">
                <div>
                  <div className="tool-row__name">
                    {formatDate(report.report_date)}: {report.expected_fill_percent}% /{" "}
                    {report.actual_fill_percent}%
                  </div>
                  <div className="tool-row__key">{t("fill.latestLabel")}</div>
                </div>
                {report.photo_view_url && (
                  <a
                    className="panel__link"
                    href={report.photo_view_url}
                    target="_blank"
                    rel="noreferrer"
                  >
                    {t("fill.photoLink")}
                  </a>
                )}
              </li>
            ))}
          </ul>
        )}
      </section>
    </div>
  );
}
