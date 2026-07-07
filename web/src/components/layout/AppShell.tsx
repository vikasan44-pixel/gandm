import { useEffect, useState } from "react";
import { Outlet } from "react-router-dom";
import { Sidebar, type NavItem } from "./Sidebar";
import { subscribeUnreadCount } from "../../notifications/poller";
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
  const nav: NavItem[] = [
    { to: "/admin/dashboard", label: t("nav.dashboard") },
    { to: "/admin/verification", label: t("nav.verification") },
    { to: "/admin/users", label: t("nav.users") },
    { to: "/admin/tools", label: t("nav.tools") },
    { to: "/admin/settings", label: t("settings.title") },
  ];
  return <Shell brand={t("app.title")} nav={nav} />;
}

export function ClientShell() {
  const nav: NavItem[] = [
    { to: "/client/cargo", label: t("nav.myCargo") },
    { to: "/client/chats", label: t("nav.chats") },
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
    { to: "/partner/chats", label: t("nav.chats") },
    { to: "/partner/notifications", label: t("nav.notifications"), badge: unreadCount },
  ];
  return <Shell brand={t("app.partnerTitle")} nav={nav} />;
}
