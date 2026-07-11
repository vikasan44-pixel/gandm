import {
  createContext,
  useContext,
  useEffect,
  useMemo,
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
  requestTokenRefresh,
  setAuthToken,
} from "../api/client";
import type { Admin, User } from "../api/types";

const STORAGE_KEY = "gandm_session";

type SessionKind = "admin" | "user";

interface StoredSession {
  kind: SessionKind;
  token: string;
  // Absent in sessions stored before refresh support shipped — those just
  // log out on the first 401, same as the old behavior.
  refreshToken?: string;
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
  logout: () => void;
}

const AuthContext = createContext<AuthContextValue | null>(null);

function loadSession(): StoredSession | null {
  try {
    const raw = localStorage.getItem(STORAGE_KEY);
    if (!raw) return null;
    const parsed = JSON.parse(raw) as StoredSession;
    if (parsed.kind !== "admin" && parsed.kind !== "user") return null;
    return parsed;
  } catch {
    return null;
  }
}

function saveSession(session: StoredSession | null) {
  if (session) {
    localStorage.setItem(STORAGE_KEY, JSON.stringify(session));
  } else {
    localStorage.removeItem(STORAGE_KEY);
  }
}

export function AuthProvider({ children }: { children: ReactNode }) {
  const [session, setSession] = useState<StoredSession | null>(null);
  const [isReady, setIsReady] = useState(false);

  useEffect(() => {
    const stored = loadSession();
    if (stored) {
      setAuthToken(stored.token);
      setSession(stored);
    }
    setIsReady(true);
  }, []);

  useEffect(() => {
    onUnauthorized(() => {
      setAuthToken(null);
      setSession(null);
      saveSession(null);
    });
  }, []);

  useEffect(() => {
    // Expired access token → exchange the refresh token for a new pair and
    // hand the fresh access token back to the API client, which replays
    // the failed request. Reads localStorage directly so the handler never
    // sees a stale React closure.
    onRefreshToken(async () => {
      const current = loadSession();
      if (!current?.refreshToken) return null;
      const tokens = await requestTokenRefresh(current.kind, current.refreshToken);
      if (!tokens) return null;
      const next: StoredSession = {
        ...current,
        token: tokens.access_token,
        refreshToken: tokens.refresh_token,
      };
      setAuthToken(next.token);
      setSession(next);
      saveSession(next);
      return tokens.access_token;
    });
  }, []);

  function applySession(next: StoredSession) {
    setAuthToken(next.token);
    setSession(next);
    saveSession(next);
  }

  async function loginAdmin(email: string, password: string) {
    const res = await apiAdminLogin(email, password);
    applySession({
      kind: "admin",
      token: res.tokens.access_token,
      refreshToken: res.tokens.refresh_token,
      admin: res.admin,
      user: null,
    });
  }

  async function loginUser(email: string, password: string): Promise<User> {
    const res = await apiUserLogin(email, password);
    applySession({
      kind: "user",
      token: res.tokens.access_token,
      refreshToken: res.tokens.refresh_token,
      admin: null,
      user: res.user,
    });
    return res.user;
  }

  // registerUser creates the account and immediately applies the issued
  // session, so the new user can upload verification documents right away.
  async function registerUser(input: RegisterInput): Promise<User> {
    const res = await apiUserRegister(input);
    applySession({
      kind: "user",
      token: res.tokens.access_token,
      refreshToken: res.tokens.refresh_token,
      admin: null,
      user: res.user,
    });
    return res.user;
  }

  function logout() {
    setAuthToken(null);
    setSession(null);
    saveSession(null);
  }

  const value = useMemo<AuthContextValue>(
    () => ({
      kind: session?.kind ?? null,
      admin: session?.admin ?? null,
      user: session?.user ?? null,
      isReady,
      loginAdmin,
      loginUser,
      registerUser,
      logout,
    }),
    [session, isReady]
  );

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
  return "/app/cargo";
}
