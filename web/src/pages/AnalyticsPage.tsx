import { useState } from "react";
import { useAsync } from "../hooks/useAsync";
import { getAnalytics } from "../api/admin";
import { LoadingState } from "../components/common/LoadingState";
import { ErrorState } from "../components/common/ErrorState";
import { t } from "../i18n";

const PERIODS = [
  { days: 7, label: "analytics.period7" },
  { days: 30, label: "analytics.period30" },
  { days: 0, label: "analytics.periodAll" },
] as const;

// AnalyticsPage (ТЗ §19.7): карточки, столбчатый график регистраций (чистый
// CSS), типы участников и топ направлений — всё из одного эндпоинта.
export function AnalyticsPage() {
  const [days, setDays] = useState<number>(7);
  const analytics = useAsync(() => getAnalytics(days), [days]);
  const data = analytics.data;

  const maxDayCount = Math.max(1, ...(data?.registrations_by_day ?? []).map((d) => d.count));
  const maxTypeCount = Math.max(1, ...(data?.participant_types ?? []).map((d) => d.count));
  const maxDirCount = Math.max(1, ...(data?.top_directions ?? []).map((d) => d.count));

  return (
    <div className="page">
      <div className="panel__header">
        <h1 className="page__title">{t("analytics.title")}</h1>
        <div className="analytics-periods">
          {PERIODS.map((p) => (
            <button
              key={p.days}
              className={"btn btn--sm " + (days === p.days ? "btn--primary" : "btn--secondary")}
              onClick={() => setDays(p.days)}
            >
              {t(p.label)}
            </button>
          ))}
        </div>
      </div>

      {analytics.isLoading && <LoadingState />}
      {analytics.error && <ErrorState message={analytics.error} onRetry={analytics.reload} />}

      {data && (
        <>
          <div className="stat-grid">
            <StatCard label={t("analytics.newUsers")} value={data.new_users} tone="green" />
            <StatCard label={t("analytics.cargoSubmitted")} value={data.cargo_submitted} tone="neutral" />
            <StatCard label={t("analytics.dealsMatched")} value={data.deals_matched} tone="yellow" />
            <StatCard label={t("analytics.verified")} value={data.verified} tone="green" />
          </div>

          <section className="panel">
            <h2 className="panel__title">{t("analytics.registrationsTitle")}</h2>
            {data.registrations_by_day.length === 0 ? (
              <p className="panel__hint">{t("analytics.registrationsEmpty")}</p>
            ) : (
              <div className="bar-chart">
                {data.registrations_by_day.map((d) => (
                  <div key={d.day} className="bar-chart__col" title={`${d.day}: ${d.count}`}>
                    <div
                      className="bar-chart__bar"
                      style={{ height: `${Math.max(4, (d.count / maxDayCount) * 120)}px` }}
                    />
                    <div className="bar-chart__label">{d.day.slice(5)}</div>
                  </div>
                ))}
              </div>
            )}
          </section>

          <div style={{ display: "grid", gridTemplateColumns: "repeat(auto-fit, minmax(280px, 1fr))", gap: 16 }}>
            <section className="panel">
              <h2 className="panel__title">{t("analytics.typesTitle")}</h2>
              <ul className="hbar-list">
                {data.participant_types.map((tc) => (
                  <li key={tc.type} className="hbar-list__row">
                    <span className="hbar-list__label">{t(`participantType.${tc.type}`)}</span>
                    <span className="hbar-list__bar" style={{ width: `${(tc.count / maxTypeCount) * 100}%` }} />
                    <span className="hbar-list__count">{tc.count}</span>
                  </li>
                ))}
              </ul>
            </section>

            <section className="panel">
              <h2 className="panel__title">{t("analytics.directionsTitle")}</h2>
              {data.top_directions.length === 0 ? (
                <p className="panel__hint">{t("analytics.directionsEmpty")}</p>
              ) : (
                <ul className="hbar-list">
                  {data.top_directions.map((dc, i) => (
                    <li key={i} className="hbar-list__row">
                      <span className="hbar-list__label" title={`${dc.origin_label} → ${dc.destination_label}`}>
                        {shorten(dc.origin_label)} → {shorten(dc.destination_label)}
                      </span>
                      <span className="hbar-list__bar" style={{ width: `${(dc.count / maxDirCount) * 100}%` }} />
                      <span className="hbar-list__count">{dc.count}</span>
                    </li>
                  ))}
                </ul>
              )}
            </section>
          </div>
        </>
      )}
    </div>
  );
}

// Геокодер отдаёт полные адреса — для списка направлений хватает первого
// сегмента («Алматы, Казахстан…» → «Алматы»).
function shorten(label: string): string {
  const first = label.split(",")[0].trim();
  return first.length > 24 ? first.slice(0, 24) + "…" : first;
}

function StatCard({ label, value, tone }: { label: string; value: number; tone: string }) {
  return (
    <div className={`stat-card stat-card--${tone}`}>
      <div className="stat-card__value">{value}</div>
      <div className="stat-card__label">{label}</div>
    </div>
  );
}
