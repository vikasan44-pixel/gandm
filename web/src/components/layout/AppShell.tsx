import { useEffect, useState } from "react";
import { Outlet } from "react-router-dom";
import { Sidebar, type NavItem } from "./Sidebar";
import { subscribeUnreadCount } from "../../notifications/poller";
import { useAuth } from "../../auth/AuthContext";
import { getMyTools } from "../../api/participant";
import { t } from "../../i18n";

function Shell({ brand, nav }: { brand: string; nav: NavItem[] }) {
  return (
    <div className="app-shell">
      <Sidebar brand={brand} nav={nav} />
      <main className="app-shell__content">
        <Outlet />
      </main>
    </div>
  );
}

export function AdminShell() {
  const { admin } = useAuth();
  // Модератор видит только свои разделы (ТЗ §19.6) — остальное скрыто в
  // навигации и в любом случае заблокировано бэкендом (403).
  const isFullAdmin = admin?.role === "admin";
  const nav: NavItem[] = [
    { to: "/admin/dashboard", label: t("nav.dashboard") },
    { to: "/admin/verification", label: t("nav.verification") },
    { to: "/admin/users", label: t("nav.users") },
    ...(isFullAdmin
      ? [
          { to: "/admin/tools", label: t("nav.tools") },
          { to: "/admin/analytics", label: t("analytics.navLabel") },
          { to: "/admin/moderators", label: t("moderators.navLabel") },
          { to: "/admin/settings", label: t("settings.title") },
        ]
      : []),
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
    getMyTools()
      .then((tools) => setToolKeys(new Set(tools.map((tl) => tl.key))))
      .catch(() => setToolKeys(new Set()));
  }, []);

  const has = (...keys: string[]) => keys.some((k) => toolKeys.has(k));

  const nav: NavItem[] = [
    { to: "/app/cargo", label: t("nav.myCargo") },
    ...(has("receive_cargo_by_route", "view_cargo_requests")
      ? [{ to: "/app/available", label: t("nav.availableCargo") }]
      : []),
    ...(has("receive_cargo_by_route")
      ? [{ to: "/app/routes", label: t("nav.routes") }]
      : []),
    ...(has("manage_warehouse_slots", "submit_fill_report")
      ? [{ to: "/app/fill-reports", label: t("fill.navLabel") }]
      : []),
    ...(has("manage_fleet")
      ? [{ to: "/app/fleet", label: t("fleet.navLabel") }]
      : []),
    ...(has("manage_fleet", "manage_warehouse_slots")
      ? [{ to: "/app/driver-competitions", label: t("driverComp.navLabel") }]
      : []),
    ...(has("manage_customs_docs")
      ? [{ to: "/app/customs", label: t("customs.navLabel") }]
      : []),
    ...(has("manage_employees")
      ? [{ to: "/app/employees", label: t("employees.navLabel") }]
      : []),
    { to: "/app/chats", label: t("nav.chats") },
    { to: "/app/rating", label: t("rating.navLabel") },
    { to: "/app/my-tools", label: t("myTools.navLabel") },
    { to: "/app/notifications", label: t("nav.notifications"), badge: unreadCount },
  ];
  return <Shell brand={t("app.partnerTitle")} nav={nav} />;
}
