import { Fragment, useEffect, useRef, useState, type CSSProperties, type KeyboardEvent as ReactKeyboardEvent } from "react";
import { createPortal } from "react-dom";
import { NavLink, useLocation } from "react-router-dom";
import { useAuth } from "../../auth/AuthContext";
import { LocaleSwitcher } from "../common/LocaleSwitcher";
import { t } from "../../i18n";

export interface NavItem {
  to: string;
  label: string;
  badge?: number;
  section?: string;
}

export interface NavGroup {
  label: string;
  items: NavItem[];
}

export function Sidebar({ brand, nav }: { brand: string; nav: NavGroup[] }) {
  const { admin, user, logout } = useAuth();
  const { pathname } = useLocation();
  const identity = admin?.email ?? user?.email ?? "";
  const visibleGroups = nav.filter((group) => group.items.length > 0);
  const activeGroupLabel = visibleGroups.find((group) =>
    group.items.some((item) => pathname === item.to || pathname.startsWith(`${item.to}/`))
  )?.label;
  const [openGroup, setOpenGroup] = useState<string | null>(null);
  const [menuPosition, setMenuPosition] = useState<{ left: number; top: number; width: number } | null>(null);
  const triggerRefs = useRef<Record<string, HTMLButtonElement | null>>({});
	const dropdownRef = useRef<HTMLDivElement>(null);

  // A selected destination closes the floating menu; the active category is
  // still highlighted in the bar, but never occupies page height.
  useEffect(() => {
    setOpenGroup(null);
    setMenuPosition(null);
  }, [pathname]);

  useEffect(() => {
    if (!openGroup) return;
    const closeOnOutside = (event: PointerEvent) => {
      const target = event.target as HTMLElement | null;
      if (target?.closest("[data-nav-dropdown]") || target?.closest("[data-nav-trigger]")) return;
      setOpenGroup(null);
      setMenuPosition(null);
    };
    const closeOnViewportChange = () => {
      setOpenGroup(null);
      setMenuPosition(null);
    };
    const closeOnEscape = (event: KeyboardEvent) => {
      if (event.key !== "Escape") return;
      const label = openGroup;
      setOpenGroup(null);
      setMenuPosition(null);
      triggerRefs.current[label]?.focus();
    };
    document.addEventListener("pointerdown", closeOnOutside);
    window.addEventListener("resize", closeOnViewportChange);
    window.addEventListener("keydown", closeOnEscape);
    return () => {
      document.removeEventListener("pointerdown", closeOnOutside);
      window.removeEventListener("resize", closeOnViewportChange);
      window.removeEventListener("keydown", closeOnEscape);
    };
  }, [openGroup]);

  const openedGroup = visibleGroups.find((group) => group.label === openGroup);

	useEffect(() => {
	  if (!openGroup || !menuPosition) return;
	  requestAnimationFrame(() => dropdownRef.current?.querySelector<HTMLElement>('[role="menuitem"]')?.focus());
	}, [openGroup, menuPosition]);

	function handleMenuKeyDown(event: ReactKeyboardEvent<HTMLDivElement>) {
	  const items = Array.from(dropdownRef.current?.querySelectorAll<HTMLElement>('[role="menuitem"]') ?? []);
	  if (items.length === 0) return;
	  const current = Math.max(0, items.indexOf(document.activeElement as HTMLElement));
	  let next: number | null = null;
	  if (event.key === "ArrowDown") next = (current + 1) % items.length;
	  if (event.key === "ArrowUp") next = (current - 1 + items.length) % items.length;
	  if (event.key === "Home") next = 0;
	  if (event.key === "End") next = items.length - 1;
	  if (event.key === "Tab") {
		setOpenGroup(null);
		setMenuPosition(null);
	  }
	  if (next !== null) {
		event.preventDefault();
		items[next].focus();
	  }
	}

  function toggleGroup(group: NavGroup, button: HTMLButtonElement) {
    if (openGroup === group.label) {
      setOpenGroup(null);
      setMenuPosition(null);
      return;
    }
    const rect = button.getBoundingClientRect();
    const width = Math.max(220, Math.min(300, rect.width + 80));
    const left = Math.max(10, Math.min(rect.left, window.innerWidth - width - 10));
    setMenuPosition({ left, top: rect.bottom + 7, width });
    setOpenGroup(group.label);
  }

  return (
    <aside className="sidebar">
      <div className="sidebar__brand">{brand}</div>
      <nav className="sidebar__nav">
        {visibleGroups.map((group) => {
          const isOpen = openGroup === group.label;
          return (
          <div className={`sidebar__group${isOpen ? " sidebar__group--open" : ""}${activeGroupLabel === group.label ? " sidebar__group--active" : ""}`} key={group.label}>
            <button
              type="button"
              className="sidebar__group-label"
              ref={(element) => { triggerRefs.current[group.label] = element; }}
              data-nav-trigger
              aria-expanded={isOpen}
              aria-haspopup="menu"
              aria-controls={isOpen ? "sidebar-floating-menu" : undefined}
              onClick={(event) => toggleGroup(group, event.currentTarget)}
            >
              <span>{group.label}</span>
              <span className="sidebar__group-chevron" aria-hidden="true">⌄</span>
            </button>
          </div>
          );
        })}
      </nav>
      {openedGroup && menuPosition && createPortal(
		<div
		  ref={dropdownRef}
          className="sidebar__dropdown"
          id="sidebar-floating-menu"
          role="menu"
		  data-nav-dropdown
		  onKeyDown={handleMenuKeyDown}
          style={{
            "--dropdown-left": `${menuPosition.left}px`,
            "--dropdown-top": `${menuPosition.top}px`,
            "--dropdown-width": `${menuPosition.width}px`,
          } as CSSProperties}
        >
          {openedGroup.items.map((item, index) => {
            const previousSection = openedGroup.items[index - 1]?.section;
            return (
              <Fragment key={`${openedGroup.label}-${item.to}`}>
                {item.section && item.section !== previousSection && (
			  <div className="sidebar__dropdown-section" role="presentation">{item.section}</div>
                )}
                <NavLink
                  to={item.to}
                  className={({ isActive }) =>
                    "sidebar__link" + (isActive ? " sidebar__link--active" : "")
                  }
                  onClick={() => setOpenGroup(null)}
                  role="menuitem"
                >
                  <span>{item.label}</span>
                  {item.badge !== undefined && item.badge > 0 && (
                    <span className="sidebar__badge">{item.badge}</span>
                  )}
                </NavLink>
              </Fragment>
            );
          })}
        </div>,
        document.body
      )}
      <div className="sidebar__footer">
        <LocaleSwitcher />
        {identity && <div className="sidebar__admin">{identity}</div>}
        <button className="btn btn--ghost btn--sm" type="button" onClick={logout}>
          {t("nav.logout")}
        </button>
      </div>
    </aside>
  );
}
