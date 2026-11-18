"use client";

// Route-group guard. The server is the source of truth for authz: this is a
// UX layer only. On mount it reads the unified session endpoint; the server
// enforces role/status on every data call regardless of what renders here.
//
//   - no session (401)      -> redirect to /login
//   - session, wrong role   -> 403 view (with a link to the user's home)
//   - session, not active   -> blocked view (suspended / pending)
//   - session, right role   -> render children
import { type ReactNode, useEffect, useState } from "react";
import Link from "next/link";
import { ShieldAlert, Clock, Loader2, ArrowRight } from "lucide-react";
import { getSession } from "@/lib/api";
import { roleHomePath, cn } from "@/lib/utils";

type GuardState = "loading" | "unauthenticated" | "forbidden" | "blocked" | "ready";

export function RouteGuard({
  requiredRole,
  children,
}: {
  requiredRole: string | string[];
  children: ReactNode;
}) {
  const [state, setState] = useState<GuardState>("loading");
  const [sessionRole, setSessionRole] = useState<string | null>(null);
  const [status, setStatus] = useState<string | null>(null);

  useEffect(() => {
    let cancelled = false;
    getSession().then((sess) => {
      if (cancelled) return;
      const allowedHere = Array.isArray(requiredRole) ? requiredRole : [requiredRole];
      if (!sess?.user) {
        setState("unauthenticated");
        // Leave the route entirely for a fresh login.
        if (typeof window !== "undefined" && !window.location.pathname.startsWith("/login")) {
          window.location.replace("/login");
        }
        return;
      }
      setSessionRole(sess.user.role);
      setStatus(sess.user.status);
      if (sess.user.status !== "active") {
        setState("blocked");
      } else if (!allowedHere.includes(sess.user.role)) {
        setState("forbidden");
      } else {
        setState("ready");
      }
    });
    return () => {
      cancelled = true;
    };
  }, [requiredRole]);

  if (state === "loading" || state === "unauthenticated") {
    return (
      <GuardShell>
        <Loader2 className="h-10 w-10 animate-spin text-slate-400" />
        <p className="text-sm font-bold uppercase tracking-widest text-slate-400">
          Verifying your session…
        </p>
      </GuardShell>
    );
  }

  if (state === "blocked") {
    return (
      <GuardShell>
        <Clock className="h-10 w-10 text-amber-500" />
        <h1 className="text-2xl font-black text-slate-800">Account {status}</h1>
        <p className="max-w-md text-slate-500">
          Your account is currently <span className="font-bold text-slate-700">{status}</span>. You can&apos;t access
          the dashboard until it is approved or reactivated.
        </p>
        <Link
          href="/login"
          className="inline-flex items-center gap-2 rounded-full bg-slate-900 px-6 py-3 text-xs font-black uppercase tracking-widest text-white hover:bg-slate-700"
        >
          Back to sign in <ArrowRight className="h-4 w-4" />
        </Link>
      </GuardShell>
    );
  }

  if (state === "forbidden") {
    return (
      <GuardShell>
        <ShieldAlert className="h-10 w-10 text-red-500" />
        <h1 className="text-2xl font-black text-slate-800">Access denied</h1>
        <p className="max-w-md text-slate-500">
          Your account doesn&apos;t have the permissions to open this area.
        </p>
        <Link
          href={roleHomePath(sessionRole)}
          className={cn(
            "inline-flex items-center gap-2 rounded-full px-6 py-3 text-xs font-black uppercase tracking-widest text-white",
            "bg-teal-600 hover:bg-teal-500"
          )}
        >
          Go to my home <ArrowRight className="h-4 w-4" />
        </Link>
      </GuardShell>
    );
  }

  return <>{children}</>;
}

function GuardShell({ children }: { children: ReactNode }) {
  return (
    <div className="flex min-h-screen w-full flex-col items-center justify-center gap-6 bg-[#FDFDFF] p-6 text-center">
      {children}
    </div>
  );
}
