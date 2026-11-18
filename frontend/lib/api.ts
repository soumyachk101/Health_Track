// Central typed API client for the Pure Go session.
//
// Auth is cookie-based: the Go backend sets HttpOnly `ht_access` / `ht_refresh`
// cookies on login/register/otp-verify. Every request here sends
// `credentials: "include"` so those cookies are attached (same-site
// subdomains `app.*`/`api.*` in production; CORS in dev). There is no Bearer
// token and no Supabase.
//
// On a 401 from a protected call we do a single-flight silent refresh
// (POST /api/v1/auth/refresh) and then retry the request exactly once. If the
// refresh itself fails (or the retry still 401s) we clear the session and
// redirect to /login.
//
// Responses can be arbitrarily shaped, so the helper types the result via T
// and uses `any` at the edge.
/* eslint-disable @typescript-eslint/no-explicit-any */

const BASE_API_URL = (process.env.NEXT_PUBLIC_API_URL || "http://127.0.0.1:8000").replace(/\/$/, "");

export const API_URL = BASE_API_URL;

export const getApiUrl = (path: string) => {
  const cleanPath = path.startsWith("/") ? path : `/${path}`;
  return `${BASE_API_URL}${cleanPath}`;
};

// Error thrown by apiFetch when the HTTP status is not 2xx. `data` carries
// the parsed response (error / message) when one was available.
export class ApiError extends Error {
  status: number;
  data: any;

  constructor(message: string, status: number, data: any) {
    super(message);
    this.name = "ApiError";
    this.status = status;
    this.data = data;
  }
}

export interface AuthFetchOptions {
  method?: "GET" | "POST" | "PUT" | "PATCH" | "DELETE";
  body?: unknown;
  // A protected (cookie-authenticated) call: on 401 we do a single silent
  // refresh + one retry. Unauthenticated entry points (login/register/...)
  // set this false so a 401 there surfaces directly instead of refreshing.
  protected?: boolean;
  // When refresh leaves us still unauthenticated, redirect to /login.
  // Default true; set false for calls that must not navigate away.
  redirectOn401?: boolean;
  signal?: AbortSignal;
}

// ---- Single-flight silent refresh ----
// Only one refresh in flight at a time so a burst of 401s does not fire a
// storm of POST /refresh (the refresh cookie rotates, so concurrent calls
// would replay a consumed token and nuke the family).
let refreshInFlight: Promise<boolean> | null = null;

export function refreshSession(): Promise<boolean> {
  if (typeof window === "undefined") return Promise.resolve(false);
  if (!refreshInFlight) {
    refreshInFlight = (async () => {
      try {
        const res = await fetch(getApiUrl("/api/v1/auth/refresh"), {
          method: "POST",
          credentials: "include",
        });
        return res.ok;
      } catch {
        return false;
      } finally {
        // Resolve before resetting so in-flight callers all share this one.
        await Promise.resolve();
        refreshInFlight = null;
      }
    })();
  }
  return refreshInFlight;
}

// Clean logout: revoke the refresh family server-side (clears both HttpOnly
// cookies), then navigate to /login. The revocation is best-effort — a failure
// to reach the endpoint must not trap the user on a broken page.
export function logout(redirectTo = "/login"): void {
  if (typeof window !== "undefined") {
    void fetch(getApiUrl("/api/v1/auth/logout"), {
      method: "POST",
      credentials: "include",
    }).catch(() => {});
    if (!window.location.pathname.startsWith(redirectTo)) {
      window.location.href = redirectTo;
    }
  }
}

async function sendRequest(
  path: string,
  method: string,
  body: unknown,
  signal?: AbortSignal
): Promise<Response> {
  const headers: Record<string, string> = {};
  if (body !== undefined) headers["Content-Type"] = "application/json";
  return fetch(getApiUrl(path), {
    method,
    headers,
    credentials: "include",
    body: body !== undefined ? JSON.stringify(body) : undefined,
    signal,
  });
}

async function parseJson(res: Response): Promise<any> {
  try {
    return await res.json();
  } catch {
    return null;
  }
}

// The data-API workhorse: cookie auth + silent 401->refresh->retry-once.
// Returns the parsed JSON body (callers type it via T).
export async function apiFetch<T = any>(path: string, opts: AuthFetchOptions = {}): Promise<T> {
  const { method = "GET", body, protected: isProtected = true, redirectOn401 = true, signal } =
    opts;

  let res = await sendRequest(path, method, body, signal);

  if (isProtected && res.status === 401) {
    const refreshed = await refreshSession();
    if (refreshed) res = await sendRequest(path, method, body, signal);
    if (res.status === 401) {
      const data = await parseJson(res);
      if (redirectOn401) logout();
      throw new ApiError(
        data?.error || "Your session has expired. Please sign in again.",
        401,
        data
      );
    }
  }

  const data = await parseJson(res);
  if (!res.ok) {
    throw new ApiError(
      data?.error || data?.message || `Request failed with status ${res.status}`,
      res.status,
      data
    );
  }
  return data as T;
}

// ---- Auth endpoints (bodies + paths match the Pure Go auth contract) ----

export interface AuthUser {
  id: number;
  username: string;
  email: string;
  first_name?: string;
  last_name?: string;
  role: string;
  status: string;
}

export interface SessionPayload {
  user: AuthUser;
  access_expires_at: string;
}

// Live session from the unified endpoint, or null when unauthenticated.
// This is the single source of truth for the app's auth state.
export async function getSession(): Promise<SessionPayload | null> {
  if (typeof window === "undefined") return null;
  try {
    const res = await fetch(getApiUrl("/api/v1/auth/session"), { credentials: "include" });
    if (!res.ok) return null;
    return await res.json();
  } catch {
    return null;
  }
}

export function apiLogin(username: string, password: string) {
  return apiFetch<{ success: boolean; user?: AuthUser; error?: string }>("/api/v1/auth/login", {
    method: "POST",
    body: { username, password },
    protected: false,
    // A wrong-creds 401 must surface as an error, not nuke-and-redirect.
    redirectOn401: false,
  });
}

export interface RegisterPayload {
  username: string;
  email: string;
  password: string;
  first_name: string;
  last_name: string;
  role?: string;
  provider_type?: string;
  business_name?: string;
  license_number?: string;
  specialization?: string;
}

export function apiRegister(payload: RegisterPayload) {
  return apiFetch<{ success: boolean; message?: string; error?: string }>("/api/v1/auth/register", {
    method: "POST",
    body: payload,
    protected: false,
    redirectOn401: false,
  });
}

export function apiOtpVerify(email: string, code: string, otpType = "register") {
  return apiFetch<{ success: boolean; user?: AuthUser; error?: string }>(
    "/api/v1/auth/otp/verify",
    { method: "POST", body: { email, code, otp_type: otpType }, protected: false, redirectOn401: false }
  );
}

// 202 {success:true} regardless of whether the email is known (no enumeration).
export function apiPasswordForgot(email: string) {
  return apiFetch<{ success: boolean; error?: string }>("/api/v1/auth/password/forgot", {
    method: "POST",
    body: { email },
    protected: false,
    redirectOn401: false,
  });
}

export function apiPasswordReset(email: string, code: string, newPassword: string) {
  return apiFetch<{ success: boolean; message?: string; error?: string }>(
    "/api/v1/auth/password/reset",
    {
      method: "POST",
      body: { email, code, new_password: newPassword },
      protected: false,
      redirectOn401: false,
    }
  );
}

export function apiGoogleLogin(credential: string) {
  return apiFetch<{ success: boolean; user?: AuthUser; error?: string }>("/api/v1/auth/google", {
    method: "POST",
    body: { credential },
    protected: false,
    redirectOn401: false,
  });
}

// ---- Protected data endpoints the pages already consume ----

export function apiProfile() {
  return apiFetch<{ user: Record<string, any>; messages: Array<{ tags: string; message: string }> }>(
    "/api/profile"
  );
}
