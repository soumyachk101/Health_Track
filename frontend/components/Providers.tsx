"use client";

import {
    createContext,
    useContext,
    useEffect,
    useState,
    useCallback,
    type ReactNode,
} from "react";
import { getSession, logout as apiLogout, type SessionPayload } from "@/lib/api";


// Re-exported for the pages/tests that imported it from here.
export { roleHomePath } from "@/lib/utils";

// The unified session endpoint is the single source of truth for auth state.
// The app no longer holds a client-side JWT — identity + role + status all come
// from `GET /api/v1/auth/session` (cookie-authenticated).
interface ProfileShape {
    user: Record<string, unknown>;
    messages: Array<{ tags: string; message: string }>;
}

interface AuthContextValue {
    // Opaque "have a live session" marker (truthy when authenticated). The app
    // does not read a token value; it only needs to know a session exists.
    token: string | null;
    profile: ProfileShape | null;
    role: string | null;
    status: string | null;
    isAuthenticated: boolean;
    // false while the initial session fetch is in flight; role/status must not
    // be trusted as "absent" until this is true.
    ready: boolean;
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
    const [session, setSession] = useState<SessionPayload | null>(null);
    const [ready, setReady] = useState(false);

    // Hydrate from the unified session endpoint on mount.
    useEffect(() => {
        let cancelled = false;
        getSession().then((s) => {
            if (cancelled) return;
            setSession(s);
            setReady(true);
        });
        return () => {
            cancelled = true;
        };
    }, []);

    const logout = useCallback(() => {
        setSession(null);
        // Revoke the refresh family server-side and redirect to /login.
        apiLogout();
    }, []);

    const role = session?.user.role ?? null;
    const status = session?.user.status ?? null;
    const token = session ? String(session.user.id) : null;
    const profile: ProfileShape | null = session
        ? { user: session.user as unknown as Record<string, unknown>, messages: [] }
        : null;

    const value: AuthContextValue = {
        token: ready ? token : null,
        profile: ready ? profile : null,
        role,
        status,
        isAuthenticated: !!token && ready,
        ready,
        logout,
    };

    return (
        <AuthContext.Provider value={value}>
            {children}
        </AuthContext.Provider>
    );
}

// Legacy client-side gate, now driven by the unified session state. Real
// authorization is enforced server-side; RouteGuard (in the group layouts)
// owns the redirect UX. This is a thin, redirect-free render gate.
export function RequireAuth({
    children,
    requireRole,
}: {
    children: ReactNode;
    requireRole?: string | string[];
}) {
    const { token, role, status, ready } = useAuth();

    if (!ready || !token || status !== "active") return null;
    if (requireRole) {
        const allowed = Array.isArray(requireRole) ? requireRole : [requireRole];
        if (!role || !allowed.includes(role)) return null;
    }
    return <>{children}</>;
}

export default function Providers({ children }: { children: ReactNode }) {
    return <AuthProvider>{children}</AuthProvider>;
}
