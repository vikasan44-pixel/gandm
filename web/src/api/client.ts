import { t } from "../i18n";

const API_BASE_URL = import.meta.env.VITE_API_BASE_URL ?? "/api";

export class ApiError extends Error {
  code: string;
  status: number;

  constructor(status: number, code: string, message: string) {
    super(message);
    this.name = "ApiError";
    this.code = code;
    this.status = status;
  }
}

type Listener = () => void;
type RefreshHandler = () => Promise<string | null>;

let authToken: string | null = null;
let unauthorizedListener: Listener | null = null;
let refreshHandler: RefreshHandler | null = null;
let refreshInFlight: Promise<string | null> | null = null;

export function setAuthToken(token: string | null) {
  authToken = token;
}

export function onUnauthorized(listener: Listener) {
  unauthorizedListener = listener;
}

// onRefreshToken registers the session-refresh callback (set by
// AuthContext, which owns the refresh token). It must return the new
// access token, or null if the session can't be refreshed.
export function onRefreshToken(handler: RefreshHandler) {
  refreshHandler = handler;
}

// tryRefresh is single-flight: concurrent 401s (e.g. a page firing several
// requests after the access token expired) share one refresh call.
function tryRefresh(): Promise<string | null> {
  if (!refreshHandler) return Promise.resolve(null);
  if (!refreshInFlight) {
    refreshInFlight = refreshHandler().finally(() => {
      refreshInFlight = null;
    });
  }
  return refreshInFlight;
}

// requestTokenRefresh is the raw refresh call — used by AuthContext's
// handler. Deliberately bypasses request() so a 401 here can't recurse.
export async function requestTokenRefresh(
  kind: "admin" | "user",
  refreshToken: string
): Promise<{ access_token: string; refresh_token: string } | null> {
  const path = kind === "admin" ? "/admin/refresh" : "/refresh";
  try {
    const res = await fetch(`${API_BASE_URL}${path}`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ refresh_token: refreshToken }),
    });
    if (!res.ok) return null;
    const data = (await res.json()) as {
      tokens?: { access_token?: string; refresh_token?: string };
    };
    if (!data.tokens?.access_token || !data.tokens.refresh_token) return null;
    return { access_token: data.tokens.access_token, refresh_token: data.tokens.refresh_token };
  } catch {
    return null;
  }
}

interface ErrorBody {
  error?: { code?: string; message?: string };
}

async function request<T>(path: string, options: RequestInit = {}): Promise<T> {
  // Headers are rebuilt per attempt so a retry after refresh picks up the
  // new access token.
  const doFetch = async (): Promise<Response> => {
    const headers = new Headers(options.headers);
    // FormData must NOT get an explicit Content-Type — the browser sets
    // multipart/form-data with the boundary itself.
    if (options.body && !(options.body instanceof FormData) && !headers.has("Content-Type")) {
      headers.set("Content-Type", "application/json");
    }
    if (authToken) {
      headers.set("Authorization", `Bearer ${authToken}`);
    }
    try {
      return await fetch(`${API_BASE_URL}${path}`, { ...options, headers });
    } catch {
      throw new ApiError(
        0,
        "network_error",
        "Не удалось связаться с сервером. Проверьте, что backend запущен."
      );
    }
  };

  let res = await doFetch();

  // Expired access token: refresh once and replay the request. Only if the
  // refresh also fails do we treat the session as dead.
  if (res.status === 401) {
    const newToken = await tryRefresh();
    if (newToken) {
      res = await doFetch();
    }
    if (res.status === 401) {
      unauthorizedListener?.();
    }
  }

  if (!res.ok) {
    let code = "unknown_error";
    let message = `Ошибка запроса (HTTP ${res.status})`;
    try {
      const data = (await res.json()) as ErrorBody;
      if (data.error) {
        code = data.error.code ?? code;
        message = data.error.message ?? message;
      }
    } catch {
      // response body wasn't JSON — keep the generic message above
    }
    // Server error messages are English; show the localized text for known
    // codes and fall back to the raw server message for unknown ones.
    const localized = t(`apiErrors.${code}`);
    if (localized !== `apiErrors.${code}`) {
      message = localized;
    }
    throw new ApiError(res.status, code, message);
  }

  if (res.status === 204) {
    return undefined as T;
  }
  return (await res.json()) as T;
}

export const api = {
  get: <T>(path: string) => request<T>(path, { method: "GET" }),
  post: <T>(path: string, body?: unknown) =>
    request<T>(path, {
      method: "POST",
      body: body !== undefined ? JSON.stringify(body) : undefined,
    }),
  patch: <T>(path: string, body?: unknown) =>
    request<T>(path, {
      method: "PATCH",
      body: body !== undefined ? JSON.stringify(body) : undefined,
    }),
  put: <T>(path: string, body?: unknown) =>
    request<T>(path, {
      method: "PUT",
      body: body !== undefined ? JSON.stringify(body) : undefined,
    }),
  del: <T>(path: string) => request<T>(path, { method: "DELETE" }),
  // multipart POST: no Content-Type header — the browser sets the boundary.
  postForm: <T>(path: string, form: FormData) =>
    request<T>(path, { method: "POST", body: form }),
};
