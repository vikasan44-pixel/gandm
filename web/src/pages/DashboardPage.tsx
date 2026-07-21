import { useMemo, useState } from "react";
import { Link } from "react-router-dom";
import { useAsync } from "../hooks/useAsync";
import {
  approveVerification,
  getAuditLog,
  getDashboardStats,
  getVerificationQueue,
  rejectVerification,
} from "../api/admin";
import { LoadingState } from "../components/common/LoadingState";
import { ErrorState } from "../components/common/ErrorState";
import { EmptyState } from "../components/common/EmptyState";
import { DetailModal } from "../components/common/DetailModal";
import { ApiError } from "../api/client";
import { formatDateTime } from "../utils/date";
import { t } from "../i18n";

function auditActionLabel(action: string): string {
  const label = t(`auditActions.${action}`);
  return label === `auditActions.${action}` ? action : label;
}

export function DashboardPage() {
  const stats = useAsync(getDashboardStats, []);
  const pendingQueue = useAsync(() => getVerificationQueue("pending"), []);
  const auditFeed = useAsync(() => getAuditLog(10), []);

  const [actionError, setActionError] = useState<string | null>(null);
  const [processingId, setProcessingId] = useState<string | null>(null);
	const [rejectingId, setRejectingId] = useState<string | null>(null);
	const [rejectReason, setRejectReason] = useState("");
	const [rejectError, setRejectError] = useState<string | null>(null);

  const topUrgent = useMemo(() => (pendingQueue.data ?? []).slice(0, 3), [pendingQueue.data]);

  async function handleApprove(id: string) {
    setActionError(null);
    setProcessingId(id);
    try {
      await approveVerification(id);
      pendingQueue.reload();
      auditFeed.reload();
    } catch (err) {
      setActionError(err instanceof ApiError ? err.message : t("common.unexpectedError"));
    } finally {
      setProcessingId(null);
    }
  }

	async function handleReject() {
	  if (!rejectingId) return;
	  const reason = rejectReason.trim();
	  if (!reason) {
		setRejectError(t("verification.reasonRequired"));
		return;
	  }
	  setActionError(null);
	  setRejectError(null);
	  setProcessingId(rejectingId);
	  try {
		await rejectVerification(rejectingId, reason);
		pendingQueue.reload();
		auditFeed.reload();
		setRejectingId(null);
		setRejectReason("");
	  } catch (err) {
		setRejectError(err instanceof ApiError ? err.message : t("common.unexpectedError"));
    } finally {
      setProcessingId(null);
    }
  }

  return (
    <div className="page">
      <h1 className="page__title">{t("nav.dashboard")}</h1>

      {stats.isLoading && <LoadingState />}
      {stats.error && <ErrorState message={stats.error} onRetry={stats.reload} />}
      {stats.data && (
        <div className="stat-cards">
          <div className="stat-card stat-card--red">
            <div className="stat-card__value">{stats.data.waiting_verification}</div>
            <div className="stat-card__label">{t("dashboard.waitingVerification")}</div>
          </div>
          <div className="stat-card stat-card--yellow">
            <div className="stat-card__value">{stats.data.new_today}</div>
            <div className="stat-card__label">{t("dashboard.newToday")}</div>
          </div>
          <div className="stat-card stat-card--green">
            <div className="stat-card__value">{stats.data.active_users}</div>
            <div className="stat-card__label">{t("dashboard.activeUsers")}</div>
          </div>
          <div className="stat-card stat-card--neutral">
            <div className="stat-card__value">{stats.data.visits}</div>
            <div className="stat-card__label">{t("dashboard.visits")}</div>
          </div>
        </div>
      )}

      <section className="panel">
        <h2 className="panel__title">{t("dashboard.urgentTitle")}</h2>
        {pendingQueue.isLoading && <LoadingState />}
        {pendingQueue.error && (
          <ErrorState message={pendingQueue.error} onRetry={pendingQueue.reload} />
        )}
        {pendingQueue.data && topUrgent.length === 0 && (
          <EmptyState message={t("dashboard.noUrgent")} />
        )}
        {topUrgent.length > 0 && (
          <ul className="urgent-list">
            {topUrgent.map((item) => (
              <li key={item.verification_id} className="urgent-list__item">
                <div>
                  <div className="urgent-list__name">{item.company_name || item.email}</div>
                  <div className="urgent-list__meta">{formatDateTime(item.created_at)}</div>
                </div>
                <div className="urgent-list__actions">
                  <button
                    className="btn btn--primary btn--sm"
                    disabled={processingId === item.verification_id}
                    onClick={() => handleApprove(item.verification_id)}
                  >
                    {t("verification.approve")}
                  </button>
                  <button
                    className="btn btn--secondary btn--sm"
                    disabled={processingId === item.verification_id}
					onClick={() => {
					  setRejectingId(item.verification_id);
					  setRejectReason("");
					  setRejectError(null);
					}}
                  >
                    {t("verification.reject")}
                  </button>
                </div>
              </li>
            ))}
          </ul>
        )}
        {actionError && <div className="form-error">{actionError}</div>}
        <Link className="panel__link" to="/admin/verification">
          {t("dashboard.goToQueue")}
        </Link>
      </section>

      <section className="panel">
        <h2 className="panel__title">{t("dashboard.activityTitle")}</h2>
        {auditFeed.isLoading && <LoadingState />}
        {auditFeed.error && <ErrorState message={auditFeed.error} onRetry={auditFeed.reload} />}
        {auditFeed.data && auditFeed.data.length === 0 && (
          <EmptyState message={t("dashboard.noActivity")} />
        )}
        {auditFeed.data && auditFeed.data.length > 0 && (
          <ul className="activity-feed">
            {auditFeed.data.map((entry) => (
              <li key={entry.id} className="activity-feed__item">
                <span className="pill pill--neutral">{auditActionLabel(entry.action)}</span>
                <span className="activity-feed__name">
                  {entry.target_label ?? ""}
                  {entry.target_label ? " — " : ""}
                  {entry.admin_email}
                </span>
                <span className="activity-feed__date">{formatDateTime(entry.created_at)}</span>
              </li>
            ))}
          </ul>
        )}
	  </section>

	  {rejectingId && (
		<DetailModal onClose={() => setRejectingId(null)}>
		  <h2 className="panel__title">{t("verification.reject")}</h2>
		  <label className="field">
			<span className="field__label">{t("verification.rejectPrompt")}</span>
			<textarea value={rejectReason} placeholder={t("verification.reasonPlaceholder")} onChange={(event) => setRejectReason(event.target.value)} />
		  </label>
		  {rejectError && <div className="form-error">{rejectError}</div>}
		  <div className="modal-actions">
			<button className="btn btn--secondary" type="button" onClick={() => setRejectingId(null)}>{t("common.cancel")}</button>
			<button className="btn btn--danger" type="button" disabled={processingId === rejectingId} onClick={() => void handleReject()}>{t("verification.confirmReject")}</button>
		  </div>
		</DetailModal>
	  )}
	</div>
  );
}
