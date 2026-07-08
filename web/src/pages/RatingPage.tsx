import { useAsync } from "../hooks/useAsync";
import { getMyReceivedRatings, getUserRating } from "../api/participant";
import { useAuth } from "../auth/AuthContext";
import { LoadingState } from "../components/common/LoadingState";
import { ErrorState } from "../components/common/ErrorState";
import { EmptyState } from "../components/common/EmptyState";
import { formatDateTime } from "../utils/date";
import { t } from "../i18n";

// The "profile rating" view: my average + received ratings. Shared by the
// client and partner cabinets.
export function RatingPage() {
  const { user } = useAuth();
  const summary = useAsync(
    () => getUserRating(user?.id ?? ""),
    [user?.id]
  );
  const received = useAsync(getMyReceivedRatings, []);

  return (
    <div className="page">
      <h1 className="page__title">{t("rating.title")}</h1>

      <section className="panel">
        <h2 className="panel__title">{t("rating.myAverage")}</h2>
        {summary.isLoading && <LoadingState />}
        {summary.error && <ErrorState message={summary.error} onRetry={summary.reload} />}
        {summary.data && (
          <div className="stat-card stat-card--neutral">
            <div className="stat-card__value">
              {summary.data.average !== null ? `★ ${summary.data.average}` : "—"}
            </div>
            <div className="stat-card__label">
              {summary.data.count > 0
                ? `${summary.data.count} ${t("rating.ratingsCount")}`
                : t("rating.noRatings")}
            </div>
          </div>
        )}
      </section>

      <section className="panel">
        <h2 className="panel__title">{t("rating.receivedTitle")}</h2>
        {received.isLoading && <LoadingState />}
        {received.error && <ErrorState message={received.error} onRetry={received.reload} />}
        {received.data && received.data.length === 0 && (
          <EmptyState message={t("rating.noReceived")} />
        )}
        {received.data && received.data.length > 0 && (
          <ul className="activity-feed">
            {received.data.map((rating) => (
              <li key={rating.id} className="activity-feed__item">
                <span className="pill pill--yellow">★ {rating.score}</span>
                <span className="activity-feed__name">{rating.comment ?? ""}</span>
                <span className="activity-feed__date">{formatDateTime(rating.created_at)}</span>
              </li>
            ))}
          </ul>
        )}
      </section>
    </div>
  );
}
