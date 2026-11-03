// Generic JSON API boundary: responses can be arbitrarily shaped, so the
// helper intentionally uses `any` at the edge and lets callers type via T.
/* eslint-disable @typescript-eslint/no-explicit-any */
import { getStoredToken, clearSession } from "./utils";

const BASE_API_URL = (process.env.NEXT_PUBLIC_API_URL || "http://127.0.0.1:8000").replace(/\/$/, "");

export const getApiUrl = (path: string) => {
  const cleanPath = path.startsWith("/") ? path : `/${path}`;
  return `${BASE_API_URL}${cleanPath}`;
};

export const API_URL = BASE_API_URL;

export interface AuthUser {
  id: number;
  username: string;
  email: string;
  role: string;
}

// Error thrown by apiFetch when the HTTP status is not 2xx or the body is
// {success:false}. `data` carries the parsed response (error message,
// requires_verification, ...) when one was available.
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

export interface ApiFetchOptions {
  method?: "GET" | "POST" | "PUT" | "PATCH" | "DELETE";
  body?: unknown;
  // Whether to attach the stored Bearer token. Default true.
  auth?: boolean;
  // On 401 from a protected call: "redirect" clears the session and navigates
  // to /login (unless already there); "ignore" only clears the session.
  onUnauthorized?: "redirect" | "ignore";
}

// Central fetch helper: prefixes via getApiUrl, sends JSON, attaches the
// Bearer token from localStorage, and returns parsed JSON. The Go backend
// has no CSRF protection, so no X-CSRFToken handling is done here.
export async function apiFetch<T = any>(path: string, opts: ApiFetchOptions = {}): Promise<T> {
  const { method = "GET", body, auth = true, onUnauthorized = "redirect" } = opts;

  const headers: Record<string, string> = {};
  if (body !== undefined) headers["Content-Type"] = "application/json";
  if (auth) {
    const token = getStoredToken();
    if (token) headers["Authorization"] = `Bearer ${token}`;
  }

  const res = await fetch(getApiUrl(path), {
    method,
    headers,
    body: body !== undefined ? JSON.stringify(body) : undefined,
  });

  let data: any = null;
  try {
    data = await res.json();
  } catch {
    // Non-JSON body.
  }

  if (res.status === 401 && auth) {
    clearSession();
    if (
      onUnauthorized === "redirect" &&
      typeof window !== "undefined" &&
      !window.location.pathname.startsWith("/login")
    ) {
      window.location.href = "/login";
    }
    throw new ApiError(data?.error || "Unauthorized", res.status, data);
  }

  const successAware = data && typeof data === "object" && "success" in data;
  if (!res.ok || (successAware && data.success === false)) {
    throw new ApiError(
      data?.error || `Request failed with status ${res.status}`,
      res.status,
      data
    );
  }

  return data as T;
}

// ---- Auth endpoints (bodies match the Go backend exactly) ----

export function apiLogin(username: string, password: string) {
  return apiFetch<{ success: boolean; token: string; user: AuthUser }>("/accounts/api/login", {
    method: "POST",
    body: { username, password },
    auth: false,
  });
}

export interface RegisterPayload {
  username: string;
  email: string;
  password: string;
  first_name: string;
  last_name: string;
  role: string;
  provider_type: string;
  business_name: string;
  license_number: string;
  specialization: string;
  state: string;
}

export function apiRegister(payload: RegisterPayload) {
  return apiFetch<{ success: boolean; email?: string; username?: string; message?: string; error?: string }>(
    "/accounts/api/register",
    { method: "POST", body: payload, auth: false }
  );
}

export function apiVerifyOtp(otp: string, email: string, otpType = "register") {
  return apiFetch<{ success: boolean; token?: string; user?: AuthUser; error?: string }>("/accounts/api/verify-otp", {
    method: "POST",
    body: { otp, email, otp_type: otpType },
    auth: false,
  });
}

export function apiResendOtp(otpType: string, email: string, firstName = "") {
  return apiFetch<{ success: boolean; message?: string; error?: string }>("/accounts/api/resend-otp", {
    method: "POST",
    body: { otp_type: otpType, email, first_name: firstName },
    auth: false,
  });
}

export function apiForgotPassword(email: string) {
  // 200 {success:true} even for unknown emails (no enumeration); 500 on server failure.
  return apiFetch<{ success: boolean; error?: string }>("/accounts/api/forgot-password", {
    method: "POST",
    body: { email },
    auth: false,
  });
}

export function apiResetPassword(email: string, otp: string, newPassword: string) {
  return apiFetch<{ success: boolean; message?: string; error?: string }>("/accounts/api/reset-password", {
    method: "POST",
    body: { email, otp, new_password: newPassword },
    auth: false,
  });
}

export function apiGoogleLogin(credential: string, role = "patient") {
  return apiFetch<{ success: boolean; token?: string; user?: AuthUser; error?: string }>("/accounts/api/google-login", {
    method: "POST",
    body: { credential, role },
    auth: false,
  });
}

export function apiProfile() {
  return apiFetch<{ user: Record<string, any>; messages: Array<{ tags: string; message: string }> }>("/api/profile");
}

// ---- Supabase onboarding: capture role/provider fields server-side ----
// Called AFTER Supabase signup so the Go backend can mirror the user into its
// profiles table with the app-specific fields (role, provider_type, ...). The
// endpoint is being added in parallel with this frontend work; when it 404s
// (not deployed yet) this logs and continues instead of crashing the signup
// flow — the trigger-based profile row already exists server-side.
export interface RegisterProfilePayload {
  email: string;
  role: string;
  provider_type?: string;
  business_name?: string;
  license_number?: string;
  specialization?: string;
  first_name?: string;
  last_name?: string;
  [key: string]: unknown;
}

export async function apiRegisterProfile(payload: RegisterProfilePayload): Promise<{
  ok: boolean;
  status?: number;
}> {
  try {
    await apiFetch("/accounts/api/register-profile", {
      method: "POST",
      body: payload,
      auth: false,
      // 401/404 must not clear the (just-created) Supabase session or
      // redirect the user away from the success state.
      onUnauthorized: "ignore",
    });
    return { ok: true };
  } catch (err) {
    const status = err instanceof ApiError ? err.status : undefined;
    if (status === 404) {
      console.warn(
        "[api] /accounts/api/register-profile not deployed yet (404); continuing signup without it."
      );
    } else {
      console.warn("[api] register-profile call failed:", err instanceof Error ? err.message : err);
    }
    return { ok: false, status };
  }
}
