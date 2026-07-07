import { useState } from "react";
import { useAsync } from "../hooks/useAsync";
import {
  approveVerification,
  getVerificationDetail,
  getVerificationQueue,
  rejectVerification,
} from "../api/admin";
import { LoadingState } from "../components/common/LoadingState";
import { ErrorState } from "../components/common/ErrorState";
import { EmptyState } from "../components/common/EmptyState";
import { UrgencyPill } from "../components/common/StatusPill";
import { ApiError } from "../api/client";
import { formatDateTime, verificationUrgency } from "../utils/date";
import { t } from "../i18n";
import type { DocumentView } from "../api/types";

export function VerificationPage() {
  const queue = useAsync(() => getVerificationQueue("pending"), []);
  const [selectedId, setSelectedId] = useState<string | null>(null);

  return (
    <div className="page page--split">
      <div className="page__list">
        <h1 className="page__title">{t("verification.title")}</h1>
        {queue.isLoading && <LoadingState />}
        {queue.error && <ErrorState message={queue.error} onRetry={queue.reload} />}
        {queue.data && queue.data.length === 0 && (
          <EmptyState message={t("verification.empty")} />
        )}
        {queue.data && queue.data.length > 0 && (
          <ul className="queue-list">
            {queue.data.map((item) => (
              <li
                key={item.verification_id}
                className={
                  "queue-list__item" +
                  (selectedId === item.verification_id ? " queue-list__item--active" : "")
                }
                onClick={() => setSelectedId(item.verification_id)}
              >
                <div className="queue-list__main">
                  <div className="queue-list__name">{item.company_name || item.email}</div>
                  <div className="queue-list__meta">{formatDateTime(item.created_at)}</div>
                </div>
                <UrgencyPill urgency={verificationUrgency(item.created_at)} />
              </li>
            ))}
          </ul>
        )}
      </div>
      <div className="page__detail">
        {selectedId ? (
          <VerificationDetailPanel
            verificationId={selectedId}
            onResolved={() => {
              setSelectedId(null);
              queue.reload();
            }}
          />
        ) : (
          <EmptyState message={t("verification.selectHint")} />
        )}
      </div>
    </div>
  );
}

function VerificationDetailPanel({
  verificationId,
  onResolved,
}: {
  verificationId: string;
  onResolved: () => void;
}) {
  const detail = useAsync(() => getVerificationDetail(verificationId), [verificationId]);
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [actionError, setActionError] = useState<string | null>(null);
  const [isRejecting, setIsRejecting] = useState(false);
  const [rejectReason, setRejectReason] = useState("");

  async function handleApprove() {
    setActionError(null);
    setIsSubmitting(true);
    try {
      await approveVerification(verificationId);
      onResolved();
    } catch (err) {
      setActionError(err instanceof ApiError ? err.message : t("common.unexpectedError"));
    } finally {
      setIsSubmitting(false);
    }
  }

  async function handleReject() {
    if (!rejectReason.trim()) {
      setActionError(t("verification.reasonRequired"));
      return;
    }
    setActionError(null);
    setIsSubmitting(true);
    try {
      await rejectVerification(verificationId, rejectReason.trim());
      onResolved();
    } catch (err) {
      setActionError(err instanceof ApiError ? err.message : t("common.unexpectedError"));
    } finally {
      setIsSubmitting(false);
    }
  }

  if (detail.isLoading) return <LoadingState />;
  if (detail.error) return <ErrorState message={detail.error} onRetry={detail.reload} />;
  if (!detail.data) return null;

  const { user, verification, documents } = detail.data;

  return (
    <div className="detail-panel">
      <h2 className="detail-panel__title">{user.company_name || user.email}</h2>
      <dl className="detail-panel__fields">
        <div>
          <dt>{t("users.email")}</dt>
          <dd>{user.email}</dd>
        </div>
        <div>
          <dt>{t("users.phone")}</dt>
          <dd>{user.phone || "—"}</dd>
        </div>
        <div>
          <dt>{t("users.type")}</dt>
          <dd>{t(`participantType.${user.participant_type}`)}</dd>
        </div>
        <div>
          <dt>{t("verification.submittedAt")}</dt>
          <dd>{formatDateTime(verification.created_at)}</dd>
        </div>
      </dl>

      <h3 className="detail-panel__subtitle">{t("verification.documents")}</h3>
      {documents.length === 0 && <EmptyState message={t("verification.noDocuments")} />}
      <div className="document-list">
        {documents.map((doc) => (
          <DocumentPreview key={doc.id} document={doc} />
        ))}
      </div>

      {verification.status === "pending" ? (
        <div className="detail-panel__actions">
          <button className="btn btn--primary" onClick={handleApprove} disabled={isSubmitting}>
            {t("verification.approve")}
          </button>
          {!isRejecting ? (
            <button
              className="btn btn--secondary"
              onClick={() => setIsRejecting(true)}
              disabled={isSubmitting}
            >
              {t("verification.reject")}
            </button>
          ) : (
            <div className="reject-form">
              <textarea
                placeholder={t("verification.reasonPlaceholder")}
                value={rejectReason}
                onChange={(e) => setRejectReason(e.target.value)}
              />
              <div className="reject-form__actions">
                <button className="btn btn--danger" onClick={handleReject} disabled={isSubmitting}>
                  {t("verification.confirmReject")}
                </button>
                <button
                  className="btn btn--ghost"
                  onClick={() => setIsRejecting(false)}
                  disabled={isSubmitting}
                >
                  {t("common.cancel")}
                </button>
              </div>
            </div>
          )}
          {actionError && <div className="form-error">{actionError}</div>}
        </div>
      ) : (
        <div className="detail-panel__resolved">
          <p>
            {verification.status === "approved"
              ? t("verification.alreadyApproved")
              : t("verification.alreadyRejected")}
          </p>
          {verification.reject_reason && (
            <p className="detail-panel__reason">
              {t("verification.rejectReason")}: {verification.reject_reason}
            </p>
          )}
        </div>
      )}
    </div>
  );
}

function isImage(name: string) {
  return /\.(png|jpe?g|gif|webp)$/i.test(name);
}
function isPdf(name: string) {
  return /\.pdf$/i.test(name);
}

function DocumentPreview({ document: doc }: { document: DocumentView }) {
  return (
    <div className="document-preview">
      <div className="document-preview__header">
        <span>{t(`documentType.${doc.type}`)}</span>
        <a href={doc.view_url} target="_blank" rel="noreferrer">
          {t("verification.openInNewTab")}
        </a>
      </div>
      {isImage(doc.original_name) && (
        <img className="document-preview__media" src={doc.view_url} alt={doc.original_name} />
      )}
      {isPdf(doc.original_name) && (
        <iframe className="document-preview__media" src={doc.view_url} title={doc.original_name} />
      )}
      {!isImage(doc.original_name) && !isPdf(doc.original_name) && (
        <div className="document-preview__fallback">{doc.original_name}</div>
      )}
    </div>
  );
}
