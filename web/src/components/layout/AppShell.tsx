import { useEffect, useState } from "react";
import { Outlet } from "react-router-dom";
import { Link } from "react-router-dom";
import { Sidebar, type NavGroup } from "./Sidebar";
import { refreshUnreadCount, subscribeUnreadCount } from "../../notifications/poller";
import { useAuth } from "../../auth/AuthContext";
import { getMyTools, getNotifications, markNotificationsRead } from "../../api/participant";
import { formatDateTime } from "../../utils/date";
import { DisplayCurrencySwitcher } from "../common/DisplayCurrencySwitcher";
import { t } from "../../i18n";
import type { NotificationItem } from "../../api/types";
import { compactDirectionLabel } from "../../utils/locationLabel";

function Shell({ brand, nav, notifications }: { brand: string; nav: NavGroup[]; notifications?: { unread: number } }) {
  return (
    <div className="app-shell">
      <Sidebar brand={brand} nav={nav} />
      <div className="app-shell__main">
        {notifications && (
          <div className="app-shell__topbar">
            <DisplayCurrencySwitcher />
            <NotificationMenu unread={notifications.unread} />
          </div>
        )}
        <main className="app-shell__content">
          <Outlet />
        </main>
      </div>
    </div>
  );
}

type NotificationCategory = "cargo" | "transport" | "warehouse" | "customs";

function notificationCategory(type: string): NotificationCategory {
  if (type.includes("customs")) return "customs";
	if (type.includes("driver") || type.includes("transport")) return "transport";
  if (type.includes("batch") || type.includes("warehouse")) return "warehouse";
  return "cargo";
}

function notificationLabel(item: NotificationItem) {
  const known = t(`notifTypes.${item.type}`);
  const base = item.type === "cargo_request_available" ? t("notifications.newCargo") : known !== `notifTypes.${item.type}` ? known : t("notifications.generic");
  const direction = item.payload?.direction_label ?? (item.payload?.origin_label && item.payload?.destination_label ? `${item.payload.origin_label} → ${item.payload.destination_label}` : "");
  return direction ? `${base}: ${compactDirectionLabel(direction)}` : base;
}

function NotificationMenu({ unread }: { unread: number }) {
  const [isOpen, setIsOpen] = useState(false);
  const [items, setItems] = useState<NotificationItem[]>([]);
  const [isLoading, setIsLoading] = useState(false);

  useEffect(() => {
    if (!isOpen) return;
    setIsLoading(true);
    getNotifications()
      .then(async (data) => {
        setItems(data);
        await markNotificationsRead();
        await refreshUnreadCount();
      })
      .finally(() => setIsLoading(false));
  }, [isOpen]);

  return (
    <div className="app-notifications">
      <button className={"app-notifications__trigger" + (unread > 0 ? " app-notifications__trigger--active" : "")} type="button" aria-label={t("notifications.title")} aria-expanded={isOpen} onClick={() => setIsOpen((value) => !value)}>
        <svg className="app-notifications__icon" viewBox="0 0 24 24" width="20" height="20" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
          <path d="M18 8A6 6 0 0 0 6 8c0 7-3 9-3 9h18s-3-2-3-9" />
          <path d="M13.73 21a2 2 0 0 1-3.46 0" />
        </svg>
        {unread > 0 && <span className="app-notifications__badge">{unread > 99 ? "99+" : unread}</span>}
      </button>
      {isOpen && (
        <div className="app-notifications__dropdown">
          <div className="app-notifications__heading">
            <strong>{t("notifications.title")}</strong>
            <button type="button" aria-label={t("common.close")} onClick={() => setIsOpen(false)}>×</button>
          </div>
          {isLoading && <p className="panel__hint">{t("common.loading")}</p>}
          {!isLoading && items.length === 0 && <p className="panel__hint">{t("notifications.empty")}</p>}
          {!isLoading && (["cargo", "transport", "warehouse", "customs"] as NotificationCategory[]).map((category) => {
            const categoryItems = items.filter((item) => notificationCategory(item.type) === category);
            return (
              <section className="app-notifications__section" key={category}>
                <h3>{t(`notifications.categories.${category}`)}</h3>
                {categoryItems.length === 0 ? <p>{t("notifications.categoryEmpty")}</p> : categoryItems.slice(0, 5).map((item) => (
                  <div className="app-notifications__item" key={item.id}>
                    <span>{notificationLabel(item)}</span>
                    <time>{formatDateTime(item.created_at)}</time>
                  </div>
                ))}
              </section>
            );
          })}
          <Link className="app-notifications__all" to="/app/notifications" onClick={() => setIsOpen(false)}>{t("notifications.viewAll")}</Link>
        </div>
      )}
    </div>
  );
}

export function AdminShell() {
  const { admin } = useAuth();
  // Модератор видит только свои разделы (ТЗ §19.6) — остальное скрыто в
  // навигации и в любом случае заблокировано бэкендом (403).
  const isFullAdmin = admin?.role === "admin";
  const nav: NavGroup[] = [
    {
      label: t("navSections.administration"),
      items: [
        { to: "/admin/dashboard", label: t("nav.dashboard") },
        { to: "/admin/verification", label: t("nav.verification") },
        { to: "/admin/users", label: t("nav.users") },
        ...(isFullAdmin ? [
          { to: "/admin/tools", label: t("nav.tools") },
          { to: "/admin/analytics", label: t("analytics.navLabel") },
          { to: "/admin/moderators", label: t("moderators.navLabel") },
          { to: "/admin/settings", label: t("settings.title") },
        ] : []),
      ],
    },
  ];
  return <Shell brand={t("app.title")} nav={nav} />;
}

// MemberShell — единый кабинет участника (роли больше нет). Разделы
// показываются по инструментам, которые человек себе выбрал; «Подать
// заявку», «Чаты», «Рейтинг», «Мои инструменты» и «Уведомления» доступны
// всем. Скрытые в навигации разделы всё равно защищены бэкендом (403).
export function MemberShell() {
  const [unreadCount, setUnreadCount] = useState(0);
  const [toolKeys, setToolKeys] = useState<Set<string>>(new Set());

  useEffect(() => subscribeUnreadCount(setUnreadCount), []);
  useEffect(() => {
    let cancelled = false;
    let retryTimer: number | undefined;

    const loadTools = () => {
      getMyTools()
        .then((tools) => {
          if (!cancelled) setToolKeys(new Set(tools.map((tool) => tool.key)));
        })
        .catch(() => {
          // API может кратко быть недоступен во время обновления. Раньше
          // единственная ошибка навсегда скрывала разделы кабинета до ручной
          // перезагрузки страницы. Повторяем запрос, не затирая уже известный
          // набор инструментов.
          if (!cancelled) retryTimer = window.setTimeout(loadTools, 2000);
        });
    };

    loadTools();
    return () => {
      cancelled = true;
      if (retryTimer !== undefined) window.clearTimeout(retryTimer);
    };
  }, []);

  const has = (...keys: string[]) => keys.some((k) => toolKeys.has(k));

  const nav: NavGroup[] = [
    {
      label: t("navSections.cargo"),
      items: [{ to: "/app/available", label: t("marketplace.findCargo") }],
    },
    {
      label: t("navSections.transport"),
      items: [
        { to: "/app/search/transport", label: t("marketplace.findTransport") },
        { to: "/app/proposals", label: t("proposals.navLabel") },
      ],
    },
    {
      label: t("navSections.warehouses"),
      items: [
        { to: "/app/search/warehouses", label: t("marketplace.findWarehouse") },
        { to: "/app/warehouse-cargo", label: t("warehouseCargo.navLabel") },
      ],
    },
    {
      label: t("navSections.customsServices"),
      items: [{ to: "/app/search/customs", label: t("marketplace.findCustoms") }],
    },
    {
      label: t("navSections.competitions"),
      items: has("manage_fleet", "manage_warehouse_slots", "manage_customs_docs")
        ? [{ to: "/app/competitions", label: t("myCompetitions.navLabel") }]
        : [],
    },
    {
      label: t("navSections.cabinet"),
      items: [
        { to: "/app/cabinet", label: t("cabinet.overview") },
        {
          to: "/app/cargo",
          label: t("cabinetMenu.myCargoRequests"),
          section: t("cabinetMenu.cargoSection"),
        },
        ...(has("manage_fleet") ? [{
          to: "/app/fleet",
          label: t("cabinetMenu.myFleet"),
          section: t("cabinetMenu.transportSection"),
        }] : []),
        ...(has("receive_cargo_by_route")
          ? [{
              to: "/app/routes",
              label: t("cabinetMenu.myTransportRoutes"),
              section: t("cabinetMenu.transportSection"),
            }]
          : []),
        ...(has("manage_warehouse_slots")
          ? [{
              to: "/app/warehouses",
              label: t("warehouses.title"),
              section: t("cabinetMenu.warehouseSection"),
            }]
          : []),
        ...(has("manage_warehouse_slots")
          ? [{
              to: "/app/driver-competitions",
              label: t("cabinetMenu.myCarrierCompetitions"),
              section: t("cabinetMenu.warehouseSection"),
            }]
          : []),
        ...(has("manage_customs_docs")
          ? [{
              to: "/app/customs",
              label: t("cabinetMenu.myCustomsServices"),
              section: t("cabinetMenu.customsSection"),
            }]
          : []),
        ...(has("manage_employees")
          ? [{
              to: "/app/employees",
              label: t("cabinetMenu.myEmployees"),
              section: t("cabinetMenu.accountSection"),
            }]
          : []),
        { to: "/app/chats", label: t("nav.chats"), section: t("cabinetMenu.accountSection") },
        { to: "/app/rating", label: t("rating.navLabel"), section: t("cabinetMenu.accountSection") },
        { to: "/app/my-tools", label: t("myTools.navLabel"), section: t("cabinetMenu.accountSection") },
        {
          to: "/app/notifications",
          label: t("nav.notifications"),
          badge: unreadCount,
          section: t("cabinetMenu.accountSection"),
        },
      ],
    },
  ];
  return <Shell brand={t("app.partnerTitle")} nav={nav} notifications={{ unread: unreadCount }} />;
}
