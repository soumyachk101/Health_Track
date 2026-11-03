import { clsx, type ClassValue } from "clsx"
import { twMerge } from "tailwind-merge"

export function cn(...inputs: ClassValue[]) {
    return twMerge(clsx(inputs))
}

// Canonical session keys for the Go backend JWT.
// 'ht_token' is the standardized key (documented for all heads); the legacy
// 'token'/'user' keys are kept in sync so pre-existing pages that still read
// them keep working.
export const TOKEN_STORAGE_KEY = "ht_token"
export const ROLE_STORAGE_KEY = "ht_role"
const LEGACY_TOKEN_KEY = "token"
const LEGACY_USER_KEY = "user"

function storage(): Storage | null {
    return typeof window !== "undefined" ? window.localStorage : null
}

export function getStoredToken(): string | null {
    const s = storage()
    if (!s) return null
    return s.getItem(TOKEN_STORAGE_KEY) || s.getItem(LEGACY_TOKEN_KEY)
}

export function getSessionRole(): string | null {
    const s = storage()
    if (!s) return null
    const r = s.getItem(ROLE_STORAGE_KEY)
    if (r) return r
    // Legacy fallback: sessions written before 'ht_role' existed keep the role
    // inside the old `user` JSON (and /api/profile does not return a role).
    try {
        const legacy = JSON.parse(s.getItem(LEGACY_USER_KEY) || "null")
        return legacy && typeof legacy.role === "string" && legacy.role ? legacy.role : null
    } catch {
        return null
    }
}

// Persist the JWT + role returned by login / verify-otp / google-login.
export function setSession(token: string, role: string | null, user?: Record<string, unknown>): void {
    const s = storage()
    if (!s) return
    s.setItem(TOKEN_STORAGE_KEY, token)
    s.setItem(LEGACY_TOKEN_KEY, token)
    s.setItem(ROLE_STORAGE_KEY, role || "")
    s.setItem(LEGACY_USER_KEY, JSON.stringify({ ...(user ?? {}), role }))
}

// Drop every session key and (optionally) go back to login.
export function clearSession(redirectToLogin = false): void {
    const s = storage()
    if (!s) return
    s.removeItem(TOKEN_STORAGE_KEY)
    s.removeItem(ROLE_STORAGE_KEY)
    s.removeItem(LEGACY_TOKEN_KEY)
    s.removeItem(LEGACY_USER_KEY)
    if (redirectToLogin && typeof window !== "undefined") {
        window.location.href = "/login"
    }
}

// ---- Supabase session bridge ----
// Supabase is the identity source of truth; its RS256 access token is what we
// persist under `ht_token` (and the legacy `token`) for the data API. These
// helpers keep those keys in sync with a Supabase session without touching the
// existing Go-JWT `setSession` contract.

// Persist a Supabase access token + resolved app role under the session keys
// (canonical + legacy). Mirrors setSession's key set but takes (token, role)
// so the Supabase flow has a single place to write the app session.
export function syncSessionFromSupabase(accessToken: string, role: string | null): void {
    const s = storage()
    if (!s) return
    s.setItem(TOKEN_STORAGE_KEY, accessToken)
    s.setItem(LEGACY_TOKEN_KEY, accessToken)
    s.setItem(ROLE_STORAGE_KEY, role || "")
    // Keep the legacy `user` JSON readable (its `role` is what the legacy
    // getSessionRole fallback reads).
    s.setItem(LEGACY_USER_KEY, JSON.stringify({ role: role ?? "" }))
}

// Clear just the Supabase-managed session keys without a redirect (used by
// AuthProvider on sign-out / expired-token, before a possible redirect).
export function clearSupaSession(): void {
    const s = storage()
    if (!s) return
    s.removeItem(TOKEN_STORAGE_KEY)
    s.removeItem(ROLE_STORAGE_KEY)
    s.removeItem(LEGACY_TOKEN_KEY)
    s.removeItem(LEGACY_USER_KEY)
}

// Fast-path boot info read straight from localStorage, before the Supabase
// client hydrates (its getSession is async). Returns the stored token + role
// so the UI can render a session immediately on first paint.
export function getSupabaseSessionInfo(): {
    accessToken: string | null
    role: string | null
} {
    return {
        accessToken: getStoredToken(),
        role: getSessionRole(),
    }
}
