import { useAsync } from "../hooks/useAsync";
import {
  getFavorites,
  getMyReceivedRatings,
  getUserRating,
  removeFavorite,
} from "../api/participant";
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
          <>
            <div className="stat-card stat-card--neutral">
              <div className="stat-card__value">
                {summary.data.composite !== null ? `★ ${summary.data.composite}` : "—"}
              </div>
              <div className="stat-card__label">{t("rating.compositeLabel")}</div>
            </div>

            <h3 className="detail-panel__subtitle" style={{ marginTop: 16 }}>
              {t("rating.breakdownTitle")}
            </h3>
            <p className="panel__hint">{t("rating.breakdownHint")}</p>
            <dl className="detail-panel__fields">
              <div>
                <dt>{t("rating.componentReviews")}</dt>
                <dd>
                  {summary.data.average !== null
                    ? `★ ${summary.data.average} (${summary.data.count} ${t("rating.ratingsCount")})`
                    : t("rating.noRatings")}
                </dd>
              </div>
              <div>
                <dt>{t("rating.componentDays")}</dt>
                <dd>{summary.data.days_on_platform}</dd>
              </div>
              <div>
                <dt>{t("rating.componentDeals")}</dt>
                <dd>{summary.data.completed_deals}</dd>
              </div>
              <div>
                <dt>{t("rating.componentMessages")}</dt>
                <dd>{summary.data.chat_messages}</dd>
              </div>
              <div>
                <dt>{t("rating.componentActiveChats")}</dt>
                <dd>
                  {summary.data.chats_active} / {summary.data.chats_total}
                </dd>
              </div>
            </dl>
          </>
        )}
      </section>

      <FavoritesSection />

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

// «Избранное» (ТЗ §6.2) — честный список постоянных партнёров.
function FavoritesSection() {
  const favorites = useAsync(getFavorites, []);

  async function handleRemove(participantId: string) {
    try {
      await removeFavorite(participantId);
      favorites.reload();
    } catch {
      // список перезагрузится при следующем действии — ошибку не раздуваем
    }
  }

  return (
    <section className="panel">
      <h2 className="panel__title">{t("favorites.title")}</h2>
      <p className="panel__hint">{t("favorites.hint")}</p>
      {favorites.isLoading && <LoadingState />}
      {favorites.data && favorites.data.length === 0 && (
        <p className="panel__hint">{t("favorites.empty")}</p>
      )}
      {favorites.data && favorites.data.length > 0 && (
        <ul className="tool-group__list">
          {favorites.data.map((f) => (
            <li key={f.participant_id} className="tool-row">
              <div>
                <div className="tool-row__name">★ {f.company_name}</div>
                <div className="tool-row__key">{formatDateTime(f.created_at)}</div>
              </div>
              <button
                className="btn btn--ghost btn--sm"
                onClick={() => void handleRemove(f.participant_id)}
              >
                {t("favorites.remove")}
              </button>
            </li>
          ))}
        </ul>
      )}
    </section>
  );
}
