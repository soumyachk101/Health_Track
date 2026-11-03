"use client";

import * as React from "react"
import Link from "next/link"
import { AlertCircle, CheckCircle, Inbox, Loader2 } from "lucide-react"
import { Button } from "@/components/ui/button"
import { cn } from "@/lib/utils"

// Shared async-state primitives for data pages: consistent loading, error
// (with retry), and empty states that avoid layout shift and never fake
// success.

export function PageLoading({ label = "Loading...", className }: { label?: string; className?: string }) {
    return (
        <div
            role="status"
            aria-live="polite"
            className={cn("min-h-[60vh] flex flex-col items-center justify-center gap-3 bg-background", className)}
        >
            <Loader2 className="h-8 w-8 animate-spin text-primary" aria-hidden="true" />
            <p className="text-sm text-muted-foreground">{label}</p>
        </div>
    )
}

export function PageError({ message, onRetry }: { message: string; onRetry: () => void }) {
    return (
        <div
            role="alert"
            className="min-h-[60vh] flex flex-col items-center justify-center gap-4 bg-background px-4 text-center"
        >
            <div className="p-4 rounded-full bg-red-50 text-red-500">
                <AlertCircle className="h-8 w-8" aria-hidden="true" />
            </div>
            <div className="max-w-md space-y-1">
                <h2 className="text-lg font-semibold text-foreground">Something went wrong</h2>
                <p className="text-sm text-muted-foreground">{message}</p>
            </div>
            <Button onClick={onRetry} aria-label={`Retry: ${message}`}>
                Try again
            </Button>
        </div>
    )
}

interface PageEmptyProps {
    icon?: React.ComponentType<React.SVGProps<SVGSVGElement>>
    title: string
    description?: string
    ctaLabel?: string
    ctaHref?: string
}

export function PageEmpty({ icon: Icon = Inbox, title, description, ctaLabel, ctaHref }: PageEmptyProps) {
    return (
        <div className="rounded-3xl bg-card shadow-skeuo-lg flex flex-col items-center justify-center gap-4 p-10 text-center">
            <div className="h-14 w-14 rounded-2xl bg-primary/10 text-primary flex items-center justify-center">
                <Icon className="h-7 w-7" aria-hidden="true" />
            </div>
            <div className="space-y-1">
                <h3 className="text-base font-semibold text-foreground">{title}</h3>
                {description && <p className="text-sm text-muted-foreground max-w-sm">{description}</p>}
            </div>
            {ctaLabel && ctaHref && (
                <Link
                    href={ctaHref}
                    className="inline-flex h-12 items-center justify-center rounded-2xl px-6 py-3 text-sm font-medium bg-primary text-primary-foreground shadow-skeuo-sm hover:scale-[1.02] active:scale-95 transition-all focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2"
                >
                    {ctaLabel}
                </Link>
            )}
        </div>
    )
}

// Inline red error banner consistent with the SKEUO auth forms.
export function ErrorBanner({ children, className }: { children: React.ReactNode; className?: string }) {
    return (
        <div
            role="alert"
            className={cn("p-4 rounded-3xl bg-red-50 text-red-600 border border-red-100 flex items-start gap-3 text-sm", className)}
        >
            <AlertCircle className="h-5 w-5 shrink-0" aria-hidden="true" />
            <div>{children}</div>
        </div>
    )
}

// Inline green success banner (mirrors the error banner styling).
export function SuccessBanner({ children, className }: { children: React.ReactNode; className?: string }) {
    return (
        <div
            role="status"
            className={cn("p-4 rounded-3xl bg-emerald-50 text-emerald-700 border border-emerald-100 flex items-start gap-3 text-sm", className)}
        >
            <CheckCircle className="h-5 w-5 shrink-0" aria-hidden="true" />
            <div>{children}</div>
        </div>
    )
}

// In-layout skeleton: mirrors the "stat cards + content card" shape of the
// data pages so the shell (sidebar, header area, cards) doesn't shift when
// the real content arrives.
export function PageSkeleton({
    stats = 0,
    rows = 4,
    title = "Loading...",
}: {
    stats?: number
    rows?: number
    title?: string
}) {
    return (
        <div aria-label={title} className="space-y-6">
            <div className="space-y-2">
                <div className="h-4 w-40 rounded-lg bg-muted/50 animate-pulse" aria-hidden="true" />
            </div>
            {stats > 0 && (
                <div className={cn("grid gap-6", stats >= 4 ? "grid-cols-1 md:grid-cols-2 lg:grid-cols-4" : "grid-cols-1 md:grid-cols-3")}>
                    {Array.from({ length: stats }).map((_, i) => (
                        <div key={i} className="rounded-3xl bg-card shadow-skeuo-lg h-28 animate-pulse" aria-hidden="true" />
                    ))}
                </div>
            )}
            {rows > 0 && (
                <div className="rounded-3xl bg-card shadow-skeuo-lg p-6 space-y-4">
                    {Array.from({ length: rows }).map((_, i) => (
                        <div key={i} className="h-12 rounded-xl bg-muted/40 animate-pulse" aria-hidden="true" />
                    ))}
                </div>
            )}
        </div>
    )
}

// Field-level inline validation error. Renders nothing without a message so
// calling code can keep it in the tree; always give it a stable `id` and
// reference it from the input's aria-describedby.
export function FieldError({ id, message }: { id: string; message?: string }) {
    if (!message) return null
    return (
        <p id={id} role="alert" className="text-xs font-medium text-red-600 bg-red-50 border border-red-100 rounded-xl px-3 py-2">
            {message}
        </p>
    )
}
