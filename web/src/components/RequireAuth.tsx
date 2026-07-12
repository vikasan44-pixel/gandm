import { Navigate, Outlet } from "react-router-dom";
import { cabinetPathFor, useAuth } from "../auth/AuthContext";
import { NotFoundPage } from "../pages/NotFoundPage";

export function RequireAdmin() {
  const { kind, isReady } = useAuth();
  if (!isReady) return null;
  if (kind !== "admin") return <Navigate to="/admin/login" replace />;
  return <Outlet />;
}

// RequireMember — единый гейт кабинета: любой авторизованный участник
// (роли больше нет). Разделы внутри показываются по инструментам.
export function RequireMember() {
  const { kind, user, isReady } = useAuth();
  if (!isReady) return null;
  if (kind !== "user" || !user) return <Navigate to="/login" replace />;
  return <Outlet />;
}

// Unknown paths: send whoever is logged in to their own cabinet; guests
// (including crawlers) get a real «not found» page with noindex instead of a
// silent redirect to «/» that looked like a soft-404 to search engines.
export function HomeRedirect() {
  const { kind, user, isReady } = useAuth();
  if (!isReady) return null;
  if (kind === "admin") return <Navigate to="/admin/dashboard" replace />;
  if (kind === "user" && user) return <Navigate to={cabinetPathFor(user)} replace />;
  return <NotFoundPage />;
}
