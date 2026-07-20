import { useEffect, useState } from "react";
import { useAsync } from "../../hooks/useAsync";
import { getNotifications, markNotificationsRead } from "../../api/participant";
import { refreshUnreadCount } from "../../notifications/poller";
import { LoadingState } from "../../components/common/LoadingState";
import { ErrorState } from "../../components/common/ErrorState";
import { EmptyState } from "../../components/common/EmptyState";
import { formatDateTime } from "../../utils/date";
import { t } from "../../i18n";
import type { NotificationItem } from "../../api/types";
import { cityNameFromLabel, compactDirectionLabel } from "../../utils/locationLabel";

function notificationText(n: NotificationItem): string {
  if (n.type === "cargo_request_available" && n.payload) {
    const origin = n.payload.origin_label ?? "?";
    const destination = n.payload.destination_label ?? "?";
    return `${t("notifications.newCargo")}: ${cityNameFromLabel(origin)} → ${cityNameFromLabel(destination)}`;
  }
  // Человеческие подписи для остальных типов; технический код неизвестного
  // backend-события наружу не показываем.
  const known = t(`notifTypes.${n.type}`);
  if (known !== `notifTypes.${n.type}`) {
    const direction = n.payload?.direction_label;
    return direction ? `${known}: ${compactDirectionLabel(direction)}` : known;
  }
  return t("notifications.generic");
}

export function NotificationsPage() {
  const notifications = useAsync(getNotifications, []);
  const [hasMarkedRead, setHasMarkedRead] = useState(false);

  // Mark everything read once the list has loaded (not before — so the
  // unread dots are still visible during this visit), then poke the badge
  // poller so the sidebar counter clears immediately instead of waiting
  // for the next scheduled poll.
  useEffect(() => {
    if (hasMarkedRead || notifications.data === null) return;
    setHasMarkedRead(true);
    markNotificationsRead()
      .then(() => refreshUnreadCount())
      .catch(() => {
        // Non-critical: the next visit will retry the mark-read.
      });
  }, [notifications.data, hasMarkedRead]);

  return (
    <div className="page">
      <h1 className="page__title">{t("notifications.title")}</h1>
      <section className="panel">
        <p className="panel__hint">{t("notifications.unreadOnlyNote")}</p>
        {notifications.isLoading && <LoadingState />}
        {notifications.error && (
          <ErrorState message={notifications.error} onRetry={notifications.reload} />
        )}
        {notifications.data && notifications.data.length === 0 && (
          <EmptyState message={t("notifications.empty")} />
        )}
        {notifications.data && notifications.data.length > 0 && (
          <ul className="activity-feed">
            {notifications.data.map((n) => (
              <li key={n.id} className="activity-feed__item">
                {!n.is_read && <span className="pill pill--yellow">•</span>}
                <span className="activity-feed__name">{notificationText(n)}</span>
                <span className="activity-feed__date">{formatDateTime(n.created_at)}</span>
              </li>
            ))}
          </ul>
        )}
      </section>
    </div>
  );
}
