import { Navigate, Outlet } from "react-router-dom";
import { cabinetPathFor, useAuth } from "../auth/AuthContext";

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

// Redirect for unknown paths: send whoever is logged in to their own
// cabinet, guests to the public landing page.
export function HomeRedirect() {
  const { kind, user, isReady } = useAuth();
  if (!isReady) return null;
  if (kind === "admin") return <Navigate to="/admin/dashboard" replace />;
  if (kind === "user" && user) return <Navigate to={cabinetPathFor(user)} replace />;
  return <Navigate to="/" replace />;
}
