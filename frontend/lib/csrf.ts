"use client";

// The Go backend has no CSRF protection (no csrftoken cookie, no
// X-CSRFToken check), so nothing on the client side needs to do either.
// getCookie is kept as an inert, always-null export because older pages
// still import it; it is a no-op by design.

// eslint-disable-next-line @typescript-eslint/no-unused-vars
export function getCookie(_name: string): string | null {
    return null;
}
