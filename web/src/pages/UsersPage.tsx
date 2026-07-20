import { useEffect, useState } from "react";
import { useAsync } from "../hooks/useAsync";
import {
  addUserRoute,
  applyPermissionSet,
  blockUser,
  deleteUserRoute,
  getPermissionSets,
  getTools,
  getUserDetail,
  getUserFillReports,
  getUserRoutes,
  getUsers,
  setUserSubscription,
  setUserTools,
  unblockUser,
} from "../api/admin";
import { LoadingState } from "../components/common/LoadingState";
import { ErrorState } from "../components/common/ErrorState";
import { EmptyState } from "../components/common/EmptyState";
import { DetailModal } from "../components/common/DetailModal";
import { useConfirm } from "../components/common/ConfirmDialog";
import { UserStatusPill } from "../components/common/StatusPill";
import { GeoPointField } from "../components/geo/GeoPointField";
import { ApiError } from "../api/client";
import { formatDate, formatDateTime } from "../utils/date";
import { t } from "../i18n";
import type { GeoPoint, ParticipantType, UserStatus } from "../api/types";
import { cityLabel } from "../utils/locationLabel";
import { toolCategoryLabel } from "../utils/toolSections";

const participantTypes: ParticipantType[] = [
  "client",
  "warehouse",
  "carrier",
  "customs_rep",
];
const userStatuses: UserStatus[] = ["pending", "active", "blocked", "rejected"];

export function UsersPage() {
  const [type, setType] = useState<ParticipantType | "">("");
  const [status, setStatus] = useState<UserStatus | "">("");
  const [search, setSearch] = useState("");
  const [searchInput, setSearchInput] = useState("");
  const [selectedId, setSelectedId] = useState<string | null>(null);

  useEffect(() => {
    const timer = setTimeout(() => setSearch(searchInput), 300);
    return () => clearTimeout(timer);
  }, [searchInput]);

  const users = useAsync(() => getUsers({ type, status, search }), [type, status, search]);

  return (
    <div className="page">
      <div className="page__list">
        <h1 className="page__title">{t("users.title")}</h1>
        <div className="filters">
          <input
            className="filters__search"
            placeholder={t("users.searchPlaceholder")}
            value={searchInput}
            onChange={(e) => setSearchInput(e.target.value)}
          />
          <select value={type} onChange={(e) => setType(e.target.value as ParticipantType | "")}>
            <option value="">{t("users.allTypes")}</option>
            {participantTypes.map((pt) => (
              <option key={pt} value={pt}>
                {t(`participantType.${pt}`)}
              </option>
            ))}
          </select>
          <select value={status} onChange={(e) => setStatus(e.target.value as UserStatus | "")}>
            <option value="">{t("users.allStatuses")}</option>
            {userStatuses.map((st) => (
              <option key={st} value={st}>
                {t(`status.user.${st}`)}
              </option>
            ))}
          </select>
        </div>

        {users.isLoading && <LoadingState />}
        {users.error && <ErrorState message={users.error} onRetry={users.reload} />}
        {users.data && users.data.length === 0 && <EmptyState message={t("users.empty")} />}
        {users.data && users.data.length > 0 && (
          <div className="table-scroll">
            <table className="table">
              <thead>
                <tr>
                  <th>{t("users.columnName")}</th>
                  <th>{t("users.columnType")}</th>
                  <th>{t("users.columnStatus")}</th>
                  <th>{t("users.columnCreated")}</th>
                  <th>{t("users.columnActive")}</th>
                </tr>
              </thead>
              <tbody>
                {users.data.map((u) => (
                  <tr
                    key={u.id}
                    className={
                      "table__row" + (selectedId === u.id ? " table__row--active" : "")
                    }
                    onClick={() => setSelectedId(u.id)}
                  >
                    <td>{u.company_name || u.email}</td>
                    <td>{t(`participantType.${u.participant_type}`)}</td>
                    <td>
                      <UserStatusPill status={u.status} />
                    </td>
                    <td>{formatDate(u.created_at)}</td>
                    <td>{u.last_active_at ? formatDateTime(u.last_active_at) : "—"}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </div>
      {selectedId && (
        <DetailModal onClose={() => setSelectedId(null)} wide>
          <UserDetailPanel userId={selectedId} onChanged={() => users.reload()} />
        </DetailModal>
      )}
    </div>
  );
}

function UserDetailPanel({
  userId,
  onChanged,
}: {
  userId: string;
  onChanged: () => void;
}) {
  const confirm = useConfirm();
  const detail = useAsync(() => getUserDetail(userId), [userId]);
  const allTools = useAsync(getTools, []);
  const allSets = useAsync(getPermissionSets, []);
  const routes = useAsync(() => getUserRoutes(userId), [userId]);
  const fillReports = useAsync(() => getUserFillReports(userId), [userId]);

  const [selectedToolIds, setSelectedToolIds] = useState<string[]>([]);
  const [selectedSetId, setSelectedSetId] = useState("");
  const [routeOrigin, setRouteOrigin] = useState<GeoPoint | null>(null);
  const [routeDestination, setRouteDestination] = useState<GeoPoint | null>(null);
  const [routeFormEpoch, setRouteFormEpoch] = useState(0);
  const [isSaving, setIsSaving] = useState(false);
  const [actionError, setActionError] = useState<string | null>(null);

  useEffect(() => {
    if (detail.data) {
      setSelectedToolIds(detail.data.tools.map((tool) => tool.id));
    }
  }, [detail.data]);

  function toggleTool(id: string) {
    setSelectedToolIds((prev) =>
      prev.includes(id) ? prev.filter((x) => x !== id) : [...prev, id]
    );
  }

  async function handleSaveTools() {
    setActionError(null);
    setIsSaving(true);
    try {
      await setUserTools(userId, selectedToolIds);
      detail.reload();
      onChanged();
    } catch (err) {
      setActionError(err instanceof ApiError ? err.message : t("common.unexpectedError"));
    } finally {
      setIsSaving(false);
    }
  }

  async function handleApplySet() {
    if (!selectedSetId) return;
    setActionError(null);
    setIsSaving(true);
    try {
      await applyPermissionSet(userId, selectedSetId);
      detail.reload();
      onChanged();
    } catch (err) {
      setActionError(err instanceof ApiError ? err.message : t("common.unexpectedError"));
    } finally {
      setIsSaving(false);
    }
  }

  async function handleAddRoute() {
    if (!routeOrigin || !routeDestination || !routeOrigin.label.trim() || !routeDestination.label.trim()) {
      setActionError(t("geo.pointsRequired"));
      return;
    }
    setActionError(null);
    setIsSaving(true);
    try {
      await addUserRoute(userId, routeOrigin, routeDestination);
      setRouteOrigin(null);
      setRouteDestination(null);
      setRouteFormEpoch((v) => v + 1);
      routes.reload();
    } catch (err) {
      setActionError(err instanceof ApiError ? err.message : t("common.unexpectedError"));
    } finally {
      setIsSaving(false);
    }
  }

  async function handleDeleteRoute(routeId: string) {
    if (!await confirm({ message: t("routes.deleteConfirm"), confirmLabel: t("routes.delete") })) return;
    setActionError(null);
    setIsSaving(true);
    try {
      await deleteUserRoute(userId, routeId);
      routes.reload();
    } catch (err) {
      setActionError(err instanceof ApiError ? err.message : t("common.unexpectedError"));
    } finally {
      setIsSaving(false);
    }
  }

  async function handleBlockToggle() {
    if (!detail.data) return;
    setActionError(null);
    setIsSaving(true);
    try {
      if (detail.data.user.status === "blocked") {
        await unblockUser(userId);
      } else {
        await blockUser(userId);
      }
      detail.reload();
      onChanged();
    } catch (err) {
      setActionError(err instanceof ApiError ? err.message : t("common.unexpectedError"));
    } finally {
      setIsSaving(false);
    }
  }

  async function handleSubscriptionToggle() {
    if (!detail.data) return;
    setActionError(null);
    setIsSaving(true);
    try {
      await setUserSubscription(userId, !detail.data.user.has_subscription);
      detail.reload();
      onChanged();
    } catch (err) {
      setActionError(err instanceof ApiError ? err.message : t("common.unexpectedError"));
    } finally {
      setIsSaving(false);
    }
  }

  if (detail.isLoading) return <LoadingState />;
  if (detail.error) return <ErrorState message={detail.error} onRetry={detail.reload} />;
  if (!detail.data) return null;

  const { user, verification } = detail.data;

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
          <dt>{t("users.columnStatus")}</dt>
          <dd>
            <UserStatusPill status={user.status} />
          </dd>
        </div>
        <div>
          <dt>{t("users.columnCreated")}</dt>
          <dd>{formatDateTime(user.created_at)}</dd>
        </div>
        <div>
          <dt>{t("users.subscription")}</dt>
          <dd>{user.has_subscription ? t("users.subYes") : t("users.subNo")}</dd>
        </div>
        <div>
          <dt>{t("rating.title")}</dt>
          <dd>
            {detail.data.rating.average !== null
              ? `★ ${detail.data.rating.average} (${detail.data.rating.count} ${t("rating.ratingsCount")})`
              : t("rating.noRatings")}
          </dd>
        </div>
        {verification?.reject_reason && (
          <div>
            <dt>{t("verification.rejectReason")}</dt>
            <dd>{verification.reject_reason}</dd>
          </div>
        )}
        {detail.data.parent_company && (
          <div>
            <dt>{t("users.parentCompany")}</dt>
            <dd>
              {detail.data.parent_company.company_name || detail.data.parent_company.email}
            </dd>
          </div>
        )}
      </dl>

      {detail.data.employees && detail.data.employees.length > 0 && (
        <>
          <h3 className="detail-panel__subtitle">{t("users.employeesTitle")}</h3>
          <ul className="tool-group__list">
            {detail.data.employees.map((emp) => (
              <li key={emp.id} className="tool-row">
                <div>
                  <div className="tool-row__name">{emp.email}</div>
                  <div className="tool-row__key">{emp.phone || "—"}</div>
                </div>
                <UserStatusPill status={emp.status} />
              </li>
            ))}
          </ul>
        </>
      )}

      <div className="detail-panel__actions">
        <button className="btn btn--secondary" onClick={handleBlockToggle} disabled={isSaving}>
          {user.status === "blocked" ? t("users.unblock") : t("users.block")}
        </button>
        <button
          className="btn btn--secondary"
          onClick={handleSubscriptionToggle}
          disabled={isSaving}
        >
          {user.has_subscription ? t("users.subDisable") : t("users.subEnable")}
        </button>
      </div>

      <h3 className="detail-panel__subtitle">{t("users.applySet")}</h3>
      {allSets.isLoading && <LoadingState />}
      {allSets.error && <ErrorState message={allSets.error} onRetry={allSets.reload} />}
      {allSets.data && allSets.data.length > 0 && (
        <div className="inline-form">
          <select value={selectedSetId} onChange={(e) => setSelectedSetId(e.target.value)}>
            <option value="">{t("users.chooseSet")}</option>
            {allSets.data.map((set) => (
              <option key={set.id} value={set.id}>
                {set.name}
              </option>
            ))}
          </select>
          <button
            className="btn btn--secondary btn--sm"
            onClick={handleApplySet}
            disabled={isSaving || !selectedSetId}
          >
            {t("users.apply")}
          </button>
        </div>
      )}

      <h3 className="detail-panel__subtitle">{t("users.routesTitle")}</h3>
      {routes.isLoading && <LoadingState />}
      {routes.error && <ErrorState message={routes.error} onRetry={routes.reload} />}
      {routes.data && routes.data.length === 0 && <EmptyState message={t("users.noRoutes")} />}
      {routes.data && routes.data.length > 0 && (
        <ul className="tool-group__list">
          {routes.data.map((route) => (
            <li key={route.id} className="tool-row">
              <div>
                <div className="tool-row__name">
                  {cityLabel(route.origin)} → {cityLabel(route.destination)}
                </div>
                <div className="tool-row__key">
                  {route.origin.lat.toFixed(4)}, {route.origin.lng.toFixed(4)} →{" "}
                  {route.destination.lat.toFixed(4)}, {route.destination.lng.toFixed(4)}
                </div>
              </div>
              <button
                className="btn btn--ghost btn--sm"
                onClick={() => handleDeleteRoute(route.id)}
                disabled={isSaving}
              >
                {t("routes.delete")}
              </button>
            </li>
          ))}
        </ul>
      )}
      <div className="inline-form inline-form--stacked">
        <GeoPointField
          key={`admin-origin-${routeFormEpoch}`}
          title={t("geo.originPoint")}
          value={routeOrigin}
          onChange={setRouteOrigin}
        />
        <GeoPointField
          key={`admin-destination-${routeFormEpoch}`}
          title={t("geo.destinationPoint")}
          value={routeDestination}
          onChange={setRouteDestination}
        />
        <button
          className="btn btn--secondary btn--sm"
          onClick={handleAddRoute}
          disabled={isSaving}
        >
          {t("users.addRoute")}
        </button>
      </div>

      {fillReports.data && fillReports.data.length > 0 && (
        <>
          <h3 className="detail-panel__subtitle">{t("fill.historyTitle")}</h3>
          <ul className="tool-group__list">
            {fillReports.data.map((report) => (
              <li key={report.id} className="tool-row">
                <div className="tool-row__name">
                  {formatDate(report.report_date)}: {report.expected_fill_percent}% /{" "}
                  {report.actual_fill_percent}%
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
        </>
      )}

      <h3 className="detail-panel__subtitle">{t("users.assignedTools")}</h3>
      {allTools.isLoading && <LoadingState />}
      {allTools.error && <ErrorState message={allTools.error} onRetry={allTools.reload} />}
      {allTools.data && (
        <div className="tool-checklist">
          {allTools.data.map((tool) => (
            <label key={tool.id} className="tool-checklist__item">
              <input
                type="checkbox"
                checked={selectedToolIds.includes(tool.id)}
                onChange={() => toggleTool(tool.id)}
              />
              <span>{tool.name}</span>
              <span className="tool-checklist__category">{toolCategoryLabel(tool.category)}</span>
            </label>
          ))}
          <button className="btn btn--primary btn--sm" onClick={handleSaveTools} disabled={isSaving}>
            {t("common.save")}
          </button>
        </div>
      )}

      {actionError && <div className="form-error">{actionError}</div>}
    </div>
  );
}
