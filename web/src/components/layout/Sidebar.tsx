import { NavLink } from "react-router-dom";
import { useAuth } from "../../auth/AuthContext";
import { t } from "../../i18n";

export interface NavItem {
  to: string;
  label: string;
  badge?: number;
}

export function Sidebar({ brand, nav }: { brand: string; nav: NavItem[] }) {
  const { admin, user, logout } = useAuth();
  const identity = admin?.email ?? user?.email ?? "";

  return (
    <aside className="sidebar">
      <div className="sidebar__brand">{brand}</div>
      <nav className="sidebar__nav">
        {nav.map((item) => (
          <NavLink
            key={item.to}
            to={item.to}
            className={({ isActive }) =>
              "sidebar__link" + (isActive ? " sidebar__link--active" : "")
            }
          >
            <span>{item.label}</span>
            {item.badge !== undefined && item.badge > 0 && (
              <span className="sidebar__badge">{item.badge}</span>
            )}
          </NavLink>
        ))}
      </nav>
      <div className="sidebar__footer">
        {identity && <div className="sidebar__admin">{identity}</div>}
        <button className="btn btn--ghost btn--sm" onClick={logout}>
          {t("nav.logout")}
        </button>
      </div>
    </aside>
  );
}
