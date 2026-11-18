import { clsx, type ClassValue } from "clsx"
import { twMerge } from "tailwind-merge"

export function cn(...inputs: ClassValue[]) {
    return twMerge(clsx(inputs))
}

// Cookie-based logout helper. The Pure Go backend keeps the session in HttpOnly
// cookies, so there is nothing to persist in localStorage anymore. This just
// navigates away; `logout()` in lib/api.ts additionally revokes the refresh
// family server-side. Kept so sidebar log-out buttons have a single call site.
export function clearSession(redirectToLogin = false): void {
    if (redirectToLogin && typeof window !== "undefined") {
        window.location.href = "/login"
    }
}

// Role-gated home routes: 'admin' -> /admin, 'doctor' -> /doctor/dashboard,
// 'provider' -> /provider/dashboard, 'patient' -> /dashboard; unknown/none -> /login.
const ROLE_HOME: Record<string, string> = {
    admin: "/admin",
    doctor: "/doctor/dashboard",
    provider: "/provider/dashboard",
    patient: "/dashboard",
}

export function roleHomePath(role: string | null | undefined): string {
    return ROLE_HOME[role ?? ""] ?? "/login"
}

