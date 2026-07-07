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

let authToken: string | null = null;
let unauthorizedListener: Listener | null = null;

export function setAuthToken(token: string | null) {
  authToken = token;
}

export function onUnauthorized(listener: Listener) {
  unauthorizedListener = listener;
}

interface ErrorBody {
  error?: { code?: string; message?: string };
}

async function request<T>(path: string, options: RequestInit = {}): Promise<T> {
  const headers = new Headers(options.headers);
  if (options.body && !headers.has("Content-Type")) {
    headers.set("Content-Type", "application/json");
  }
  if (authToken) {
    headers.set("Authorization", `Bearer ${authToken}`);
  }

  let res: Response;
  try {
    res = await fetch(`${API_BASE_URL}${path}`, { ...options, headers });
  } catch {
    throw new ApiError(
      0,
      "network_error",
      "Не удалось связаться с сервером. Проверьте, что backend запущен."
    );
  }

  if (res.status === 401) {
    unauthorizedListener?.();
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
  del: <T>(path: string) => request<T>(path, { method: "DELETE" }),
};
