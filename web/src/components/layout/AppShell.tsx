import { useEffect, useState } from "react";
import { Outlet } from "react-router-dom";
import { Sidebar, type NavItem } from "./Sidebar";
import { subscribeUnreadCount } from "../../notifications/poller";
import { useAuth } from "../../auth/AuthContext";
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

export function ClientShell() {
  const nav: NavItem[] = [
    { to: "/client/cargo", label: t("nav.myCargo") },
    { to: "/client/chats", label: t("nav.chats") },
    { to: "/client/rating", label: t("rating.navLabel") },
  ];
  return <Shell brand={t("app.clientTitle")} nav={nav} />;
}

export function PartnerShell() {
  const [unreadCount, setUnreadCount] = useState(0);

  // The unread badge is fed by the app-wide singleton poller (one interval
  // per app, 30s, in-flight guard) — see notifications/poller.ts. Subscribe
  // returns its own cleanup, so unmounting never leaves a stray interval.
  useEffect(() => subscribeUnreadCount(setUnreadCount), []);

  const nav: NavItem[] = [
    { to: "/partner/cargo", label: t("nav.availableCargo") },
    { to: "/partner/routes", label: t("nav.routes") },
    { to: "/partner/fill-reports", label: t("fill.navLabel") },
    { to: "/partner/fleet", label: t("fleet.navLabel") },
    { to: "/partner/driver-competitions", label: t("driverComp.navLabel") },
    { to: "/partner/customs", label: t("customs.navLabel") },
    { to: "/partner/employees", label: t("employees.navLabel") },
    { to: "/partner/chats", label: t("nav.chats") },
    { to: "/partner/rating", label: t("rating.navLabel") },
    { to: "/partner/notifications", label: t("nav.notifications"), badge: unreadCount },
  ];
  return <Shell brand={t("app.partnerTitle")} nav={nav} />;
}
