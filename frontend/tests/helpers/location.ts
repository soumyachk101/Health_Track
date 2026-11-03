import { vi } from "vitest";

// jsdom's window.location.href is a getter-only accessor and cannot be spied
// on directly. These tests swap window.location for a plain data object via
// vi.stubGlobal, so the product code's `window.location.href = "/login"` and
// `window.location.pathname` reads hit the object's own fields and can be
// asserted on. Call `vi.unstubAllGlobals()` in an afterEach to restore.
export function stubLocation(pathname = "/") {
  const loc = {
    pathname,
    href: "http://localhost" + pathname,
    replace: (url: string) => {
      (loc as Record<string, unknown>).href = url;
    },
    // Mirror replace for code that navigates via location.assign (e.g. the
    // Google OAuth redirect on /login).
    assign: (url: string) => {
      (loc as Record<string, unknown>).href = url;
    },
  };
  vi.stubGlobal("location", loc);
  return loc as { pathname: string; href: string };
}
