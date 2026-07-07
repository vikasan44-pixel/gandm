import { Navigate, Outlet } from "react-router-dom";
import { cabinetPathFor, useAuth } from "../auth/AuthContext";

export function RequireAdmin() {
  const { kind, isReady } = useAuth();
  if (!isReady) return null;
  if (kind !== "admin") return <Navigate to="/admin/login" replace />;
  return <Outlet />;
}

export function RequireClient() {
  const { kind, user, isReady } = useAuth();
  if (!isReady) return null;
  if (kind !== "user" || !user) return <Navigate to="/login" replace />;
  if (user.participant_type !== "client") {
    return <Navigate to={cabinetPathFor(user)} replace />;
  }
  return <Outlet />;
}

export function RequirePartner() {
  const { kind, user, isReady } = useAuth();
  if (!isReady) return null;
  if (kind !== "user" || !user) return <Navigate to="/login" replace />;
  if (user.participant_type === "client") {
    return <Navigate to={cabinetPathFor(user)} replace />;
  }
  return <Outlet />;
}

// Landing redirect for "/" and unknown paths: send whoever is logged in to
// their own cabinet, everyone else to the participant login.
export function HomeRedirect() {
  const { kind, user, isReady } = useAuth();
  if (!isReady) return null;
  if (kind === "admin") return <Navigate to="/admin/dashboard" replace />;
  if (kind === "user" && user) return <Navigate to={cabinetPathFor(user)} replace />;
  return <Navigate to="/login" replace />;
}
