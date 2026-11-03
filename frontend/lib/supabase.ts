// Supabase Auth client + helpers. Supabase is the source of truth for identity;
// the Supabase-issued access token (RS256) is what the data API (lib/api.ts)
// sends as `Authorization: Bearer <token>` for the Go backend to verify via JWKS.
//
// The client is created lazily and only when the env vars are present, so a
// missing/empty env (build, SSR, or a config-not-yet-set deployment) yields
// `null` instead of throwing at module/import time. Callers get `null` from
// getSupabase() and `supaSession()`; the action helpers surface a friendly error.
/* eslint-disable @typescript-eslint/no-explicit-any */
import {
  createClient,
  AuthError,
  type SupabaseClient,
  type Session,
  type User,
} from "@supabase/supabase-js";

// Minimal identity shape we need out of Supabase. `user_metadata` carries the
// provisioning fields (role, provider_type, ...) written at signup.
export type SupaUser = {
  id: string;
  email: string;
  user_metadata?: Record<string, any>;
};

export type SupaSessionInfo = {
  accessToken: string;
  user: SupaUser;
};

// The action helpers throw this when Supabase is not configured (env missing)
// so pages can show a friendly banner instead of crashing on `null.auth`.
export class SupabaseNotConfiguredError extends Error {
  constructor() {
    super(
      "Supabase is not configured yet. Set NEXT_PUBLIC_SUPABASE_URL and NEXT_PUBLIC_SUPABASE_ANON_KEY."
    );
    this.name = "SupabaseNotConfiguredError";
  }
}

let client: SupabaseClient | null = null;
let warned = false;

/**
 * Lazy, guarded Supabase client (created once). Returns `null` when the env
 * vars are missing so the build/SSR never crashes on an absent config.
 *
 * `persistSession:false` keeps Supabase from writing its own session to
 * storage; we manage the access token ourselves under the existing
 * `ht_token` / `ht_role` keys (see lib/utils.ts).
 */
export function getSupabase(): any {
  const url = process.env.NEXT_PUBLIC_SUPABASE_URL;
  // Supabase renamed "anon key" to "publishable key" in newer projects; accept
  // either env var name so both generations of .env files work.
  const key =
    process.env.NEXT_PUBLIC_SUPABASE_ANON_KEY ||
    process.env.NEXT_PUBLIC_SUPABASE_PUBLISHABLE_KEY;
  if (!url || !key) {
    if (!warned && process.env.NODE_ENV !== "production") {
      warned = true;
      console.warn(
        "[supabase] NEXT_PUBLIC_SUPABASE_URL / NEXT_PUBLIC_SUPABASE_ANON_KEY (or _PUBLISHABLE_KEY) not set; Supabase is disabled."
      );
    }
    return null;
  }
  if (!client) {
    client = createClient(url, key, {
      auth: {
        persistSession: false,
        autoRefreshToken: true,
        detectSessionInUrl: true,
        flowType: "pkce",
      },
    });
  }
  return client;
}

// Like getSupabase(), but throws a friendly error when unset. Used by the
// action helpers that genuinely require a working client.
function requireSupabase(): any {
  const sb = getSupabase();
  if (!sb) throw new SupabaseNotConfiguredError();
  return sb;
}

// Resolve an app role from a Supabase user. The role is written to
// user_metadata at signup; legacy or google accounts may not carry it.
export function roleOfSupaUser(user: SupaUser | null | undefined): string | null {
  const role = user?.user_metadata?.role;
  return typeof role === "string" && role ? role : null;
}

// Normalize a supabase-js User into our minimal SupaUser shape (email can be
// undefined on partial users; user_metadata may be null).
function toSupaUser(user: User | null | undefined): SupaUser | null {
  if (!user) return null;
  return {
    id: user.id,
    email: user.email ?? "",
    user_metadata: (user.user_metadata ?? {}) as Record<string, any>,
  };
}

// ---- action helpers (all return the access token + user, or throw) ----

// Email/password sign-in. Resolves to the live session's access token + user.
export async function supaSignIn(
  email: string,
  password: string
): Promise<SupaSessionInfo> {
  const sb = requireSupabase();
  const { data, error } = await sb.auth.signInWithPassword({ email, password });
  if (error) throw error;
  const session = data?.session as Session | null;
  if (!session?.access_token) {
    throw new Error("Sign-in did not return a session.");
  }
  return { accessToken: session.access_token, user: toSupaUser(session.user) as SupaUser };
}

export type SupaSignUpPayload = {
  email: string;
  password: string;
  options?: { data?: Record<string, any> };
};

// Sign up. When email confirmation is OFF, Supabase returns a live session;
// when it is ON, `session` is null and the user must confirm their email.
export async function supaSignUp(
  payload: SupaSignUpPayload
): Promise<{ session: Session | null; user: SupaUser | null }> {
  const sb = requireSupabase();
  const { data, error } = await sb.auth.signUp({
    email: payload.email,
    password: payload.password,
    options: payload.options ?? {},
  });
  if (error) throw error;
  const rawUser = (data?.user as User | null) ?? null;
  // SupaUser.email is non-optional; the auth User type has it optional, so
  // normalize it here (a created user always has an email, but be safe).
  return {
    session: (data?.session as Session | null) ?? null,
    user: rawUser
      ? { id: rawUser.id, email: rawUser.email ?? "", user_metadata: rawUser.user_metadata }
      : null,
  };
}

// Sign out of the current Supabase session. No-ops when unconfigured.
export async function supaSignOut(): Promise<void> {
  const sb = getSupabase();
  if (!sb) return;
  try {
    await sb.auth.signOut();
  } catch {
    // The caller clears localStorage regardless; a failed revocation is
    // not fatal to logging the local session out.
  }
}

// Live Supabase session (in-memory; `persistSession:false` means it is only
// present after a sign-in in this client lifecycle or via detectSessionInUrl).
// Returns null when unconfigured or when there is no active session.
export async function supaSession(): Promise<SupaSessionInfo | null> {
  const sb = getSupabase();
  if (!sb) return null;
  try {
    const { data } = await sb.auth.getSession();
    const session = data?.session as Session | null;
    if (!session?.access_token) return null;
    return { accessToken: session.access_token, user: toSupaUser(session.user) as SupaUser };
  } catch {
    return null;
  }
}

// Request a password-reset email that points back at our /reset-password route.
export async function supaResetEmail(email: string, redirectTo: string): Promise<void> {
  const sb = requireSupabase();
  const { error } = await sb.auth.resetPasswordForEmail(email, { redirectTo });
  if (error) throw error;
}

// Apply the pending recovery token (established by detectSessionInUrl from the
// reset link) to set a new password.
export async function supaSetNewPassword(password: string): Promise<void> {
  const sb = requireSupabase();
  const { error } = await sb.auth.updateUser({ password });
  if (error) throw error;
}

// Google OAuth via supabase-js. Resolves to the authorization URL; the browser
// is redirected to it and returns here re-authenticated (detectSessionInUrl).
export async function supaGoogleSignIn(): Promise<{ url: string }> {
  const sb = requireSupabase();
  const { data, error } = await sb.auth.signInWithOAuth({ provider: "google" });
  if (error) throw error;
  return { url: data?.url as string };
}

// Resend the signup verification email (for accounts that require it).
export async function supaResendVerification(email: string): Promise<void> {
  const sb = requireSupabase();
  const { error } = await sb.auth.resend({ type: "signup", email });
  if (error) throw error;
}

// Subscribe to Supabase auth state so the access token stays fresh. The callback
// fires with the current session (or null after sign-out) and is invoked on
// SIGNED_IN / TOKEN_REFRESHED / SIGNED_OUT / USER_UPDATED / etc. Returns an
// unsubscribe fn. No-op (returns a no-op unsubscribe) when unconfigured.
export function watchAuth(
  cb: (session: SupaSessionInfo | null) => void
): () => void {
  const sb = getSupabase();
  if (!sb) return () => {};
  const { data } = sb.auth.onAuthStateChange((event: unknown, session: Session | null) => {
    void event;
    if (session?.access_token) {
      cb({ accessToken: session.access_token, user: session.user as SupaUser });
    } else {
      cb(null);
    }
  });
  return () => {
    data?.subscription?.unsubscribe();
  };
}

// Re-exported so pages can pattern-match `err instanceof AuthError`
// (e.g. the "email not confirmed" path on /login).
export { AuthError };
