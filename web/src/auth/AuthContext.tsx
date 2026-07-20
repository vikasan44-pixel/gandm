import {
  createContext,
  useContext,
  useEffect,
  useState,
  type ReactNode,
} from "react";
import { login as apiAdminLogin } from "../api/admin";
import {
  loginUser as apiUserLogin,
  registerUser as apiUserRegister,
  type RegisterInput,
} from "../api/participant";
import {
  onRefreshToken,
  onUnauthorized,
  requestLogout,
  requestTokenRefresh,
  setAuthToken,
} from "../api/client";
import type { Admin, User } from "../api/types";

const STORAGE_KEY = "gandm_session";

type SessionKind = "admin" | "user";

// Only non-secret profile data is persisted (who is logged in + which refresh
// endpoint to use). The access token lives in memory only, and the refresh
// token in an httpOnly cookie — neither is reachable from localStorage, so an
// XSS can't lift the session out of storage.
interface StoredProfile {
  kind: SessionKind;
  admin: Admin | null;
  user: User | null;
}

interface AuthContextValue {
  kind: SessionKind | null;
  admin: Admin | null;
  user: User | null;
  isReady: boolean;
  loginAdmin: (email: string, password: string) => Promise<void>;
  loginUser: (email: string, password: string) => Promise<User>;
  registerUser: (input: RegisterInput) => Promise<User>;
  applyUserProfile: (user: User) => void;
  logout: () => Promise<void>;
}

const AuthContext = createContext<AuthContextValue | null>(null);

function loadProfile(): StoredProfile | null {
  try {
    const raw = localStorage.getItem(STORAGE_KEY);
    if (!raw) return null;
    const parsed = JSON.parse(raw) as StoredProfile;
    if (parsed.kind !== "admin" && parsed.kind !== "user") return null;
    return parsed;
  } catch {
    return null;
  }
}

function saveProfile(profile: StoredProfile | null) {
  if (profile) {
    localStorage.setItem(STORAGE_KEY, JSON.stringify(profile));
  } else {
    localStorage.removeItem(STORAGE_KEY);
  }
}

export function AuthProvider({ children }: { children: ReactNode }) {
  const [session, setSession] = useState<StoredProfile | null>(null);
  const [isReady, setIsReady] = useState(false);

  useEffect(() => {
    // Bootstrap: the access token isn't persisted, only the httpOnly refresh
    // cookie is. If we remember who was logged in, silently refresh to mint a
    // fresh access token; if that fails the session is gone.
    let cancelled = false;
    void (async () => {
      const stored = loadProfile();
      if (stored) {
        const token = await requestTokenRefresh(stored.kind);
        if (!cancelled && token) {
          setAuthToken(token);
          setSession(stored);
        } else if (!cancelled) {
          saveProfile(null);
        }
      }
      if (!cancelled) setIsReady(true);
    })();
    return () => {
      cancelled = true;
    };
  }, []);

  useEffect(() => {
    onUnauthorized(() => {
      setAuthToken(null);
      setSession(null);
      saveProfile(null);
    });
  }, []);

  useEffect(() => {
    // Expired access token → refresh via the cookie and hand the new access
    // token back to the API client, which replays the failed request. Reads
    // localStorage directly so the handler never sees a stale React closure.
    onRefreshToken(async () => {
      const current = loadProfile();
      if (!current) return null;
      const token = await requestTokenRefresh(current.kind);
      if (!token) return null;
      setAuthToken(token);
      return token;
    });
  }, []);

  function applySession(next: StoredProfile, accessToken: string) {
    setAuthToken(accessToken);
    setSession(next);
    saveProfile(next);
  }

  async function loginAdmin(email: string, password: string) {
    const res = await apiAdminLogin(email, password);
    applySession(
      { kind: "admin", admin: res.admin, user: null },
      res.tokens.access_token
    );
  }

  async function loginUser(email: string, password: string): Promise<User> {
    const res = await apiUserLogin(email, password);
    applySession(
      { kind: "user", admin: null, user: res.user },
      res.tokens.access_token
    );
    return res.user;
  }

  // registerUser creates the account and immediately applies the issued
  // session, so the new user can upload verification documents right away.
  async function registerUser(input: RegisterInput): Promise<User> {
    const res = await apiUserRegister(input);
    applySession(
      { kind: "user", admin: null, user: res.user },
      res.tokens.access_token
    );
    return res.user;
  }

  function applyUserProfile(user: User) {
	const next: StoredProfile = { kind: "user", admin: null, user };
	setSession(next);
	saveProfile(next);
  }

  async function logout() {
    // Finish revoking/clearing the old cookie before exposing the login screen.
    // Otherwise a fast new login can set its cookie first and a delayed logout
    // response would erase that brand-new session.
    const current = loadProfile();
    if (current) await requestLogout(current.kind);
    setAuthToken(null);
    setSession(null);
    saveProfile(null);
  }

  const value: AuthContextValue = {
    kind: session?.kind ?? null,
    admin: session?.admin ?? null,
    user: session?.user ?? null,
    isReady,
    loginAdmin,
    loginUser,
    registerUser,
    applyUserProfile,
    logout,
  };

  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>;
}

export function useAuth(): AuthContextValue {
  const ctx = useContext(AuthContext);
  if (!ctx) {
    throw new Error("useAuth must be used within AuthProvider");
  }
  return ctx;
}

// Роли больше нет — один общий кабинет для всех участников. Разделы внутри
// показываются по инструментам, которые человек себе выбрал (AppShell
// фильтрует навигацию по /my/tools). Параметр оставлен для совместимости
// вызовов.
export function cabinetPathFor(_user?: User | null): string {
  return "/app/cabinet";
}
