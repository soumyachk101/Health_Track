"use client";

import {
    createContext,
    useContext,
    useEffect,
    useState,
    useCallback,
    type ReactNode,
} from "react";
import {
    setSession,
    clearSession,
    getSupabaseSessionInfo,
    syncSessionFromSupabase,
    clearSupaSession,
} from "@/lib/utils";
import { apiProfile } from "@/lib/api";
import {
    supaSession,
    supaSignOut,
    watchAuth,
    roleOfSupaUser,
} from "@/lib/supabase";

interface AuthContextValue {
    // Supabase access token stored under 'ht_token' (see lib/utils.ts).
    // The Go backend verifies this Supabase-issued JWT (ES256/RS256) via JWKS.
    token: string | null;
    // /api/profile payload (user + messages) while a token exists; null when
    // logged out or the profile fetch is still running.
    profile: Awaited<ReturnType<typeof apiProfile>> | null;
    role: string | null;
    isAuthenticated: boolean;
    // false while the initial localStorage/Supabase hydration is in flight;
    // token/role must not be trusted as "absent" until this is true.
    ready: boolean;
    setAuthed: (token: string, role: string, user?: Record<string, unknown>) => void;
    logout: () => void;
}

const AuthContext = createContext<AuthContextValue | null>(null);

export function useAuth() {
    const ctx = useContext(AuthContext);
    if (!ctx) {
        throw new Error("useAuth must be used within <Providers>");
    }
    return ctx;
}

export function AuthProvider({ children }: { children: ReactNode }) {
    const [token, setToken] = useState<string | null>(null);
    const [role, setRole] = useState<string | null>(null);
    const [profile, setProfile] = useState<Awaited<ReturnType<typeof apiProfile>> | null>(null);
    const [ready, setReady] = useState(false);

    // Hydrate from Supabase + localStorage. Supabase is the identity source of
    // truth: when a live Supabase session exists it wins and its access token
    // becomes the app session token (persisted under ht_token for the data
    // API). The fast localStorage path lets the UI render before the async
    // supaSession() resolves.
    useEffect(() => {
        let disposed = false;
        let unsubscribeAuth: (() => void) | null = null;

        const stored = getSupabaseSessionInfo();

        supaSession()
            .then(async (supaInfo) => {
                // Decide the token/role source: Supabase session first, then
                // the stored ht_token/ht_role fast path.
                let source: "supabase" | "stored" | null = null;
                let bootRole: string | null = null;

                if (supaInfo) {
                    source = "supabase";
                    bootRole = roleOfSupaUser(supaInfo.user);
                    // Keep ht_token current with the live Supabase access token.
                    syncSessionFromSupabase(supaInfo.accessToken, bootRole);
                    setToken(supaInfo.accessToken);
                } else if (stored.accessToken) {
                    source = "stored";
                    bootRole = stored.role;
                    setToken(stored.accessToken);
                }
                setRole(bootRole);

                // Best-effort profile fetch (role refinement + dashboard data).
                // A 401 clears the session and redirects; other errors leave
                // the session intact so a transient failure isn't fatal.
                let profileData: Awaited<ReturnType<typeof apiProfile>> | null = null;
                if (source) {
                    try {
                        profileData = await apiProfile();
                    } catch {
                        // 401 already redirected (outside /login) or cleared
                        // the session; other failures: continue with stored role.
                    }
                }
                if (disposed) return;

                if (profileData) setProfile(profileData);

                const profileRole = profileData?.user?.role ?? null;
                if (source === "supabase") {
                    // Metadata role > profile role > default 'patient'.
                    setRole(bootRole ?? profileRole ?? "patient");
                } else {
                    // Stored (legacy JWT) path: stored role > profile role.
                    setRole(bootRole ?? profileRole ?? null);
                }

                // Keep ht_token fresh while a Supabase session is alive.
                if (source === "supabase") {
                    unsubscribeAuth = watchAuth((s) => {
                        if (s) {
                            const r = roleOfSupaUser(s.user);
                            syncSessionFromSupabase(s.accessToken, r);
                            setToken(s.accessToken);
                            if (r) setRole(r);
                        } else {
                            // Signed out (this tab or another via Supabase):
                            // drop the local session and the cached states.
                            clearSupaSession();
                            setToken(null);
                            setRole(null);
                            setProfile(null);
                        }
                    });
                }
            })
            .catch(() => {
                // supaSession() swallows its own errors; anything here means
                // fall back to the stored session (or none) without crashing.
                if (stored.accessToken) setToken(stored.accessToken);
                if (stored.role) setRole(stored.role);
            })
            .finally(() => {
                if (!disposed) setReady(true);
            });

        return () => {
            disposed = true;
            unsubscribeAuth?.();
        };
    }, []);

    const setAuthed = useCallback((t: string, r: string, user?: Record<string, unknown>) => {
        setSession(t, r, user);
        setToken(t);
        setRole(r);
    }, []);

    const logout = useCallback(() => {
        // Sign out of Supabase (best-effort; fails silently when unset or
        // when the session was already gone), then clear the local session.
        void supaSignOut();
        clearSession(true);
        setToken(null);
        setRole(null);
        setProfile(null);
    }, []);

    const value: AuthContextValue = {
        token: ready ? token : null,
        profile: ready ? profile : null,
        role: role ?? (profile ? profile.user?.role ?? null : null),
        isAuthenticated: !!token && (ready || !!profile),
        ready,
        setAuthed,
        logout,
    };

    return (
        <AuthContext.Provider value={value}>
            {children}
        </AuthContext.Provider>
    );
}

// Role-gated route guard. Roles: 'admin' -> /admin, 'doctor' ->
// /doctor/dashboard, 'provider' -> /provider/dashboard, 'patient' ->
// /dashboard; unknown/none -> /login.
const ROLE_HOME: Record<string, string> = {
    admin: "/admin",
    doctor: "/doctor/dashboard",
    provider: "/provider/dashboard",
    patient: "/dashboard",
};

export function roleHomePath(role: string | null | undefined): string {
    return ROLE_HOME[role ?? ""] ?? "/login";
}

// Client-side gate: blocks children until authenticated and (when
// requireRole is set) until the session role is accepted. Renders nothing
// while checking.
export function RequireAuth({
    children,
    requireRole,
}: {
    children: ReactNode;
    requireRole?: string | string[];
}) {
    const { token, profile, role, isAuthenticated, ready } = useAuth();

    useEffect(() => {
        // Wait for hydration before deciding: a fresh page load has no token
        // state until Providers finishes restoring the session.
        if (!ready) return;

        // No token at all: send to login.
        if (!token) {
            if (typeof window !== "undefined" && !window.location.pathname.startsWith("/login")) {
                clearSession();
                window.location.href = "/login";
            }
            return;
        }

        const roleToCheck = profile?.user?.role ?? role;
        const allowed = Array.isArray(requireRole) ? requireRole : requireRole ? [requireRole] : null;
        if (allowed && (!roleToCheck || !allowed.includes(roleToCheck))) {
            const home = ROLE_HOME[roleToCheck ?? ""] ?? "/dashboard";
            if (typeof window !== "undefined") window.location.href = home;
        }
    }, [ready, token, profile, role, requireRole]);

    // While restoring the session, keep the guarded content out of the way
    // rather than mis-redirecting a valid visitor.
    if (!ready || !isAuthenticated) return null;

    const roleToCheck = profile?.user?.role ?? role;
    const allowed = Array.isArray(requireRole) ? requireRole : requireRole ? [requireRole] : null;
    if (allowed && (!roleToCheck || !allowed.includes(roleToCheck))) return null;

    return <>{children}</>;
}

export default function Providers({ children }: { children: ReactNode }) {
    // Google sign-in goes through Supabase (supaGoogleSignIn) rather than the
    // @react-oauth/google SDK, so no GoogleOAuthProvider wrapper is needed.
    return <AuthProvider>{children}</AuthProvider>;
}
