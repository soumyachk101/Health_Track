import { afterEach, describe, expect, it, vi } from "vitest";
import { clearSession, roleHomePath } from "@/lib/utils";

// The Pure Go cookie session lives in HttpOnly cookies, not localStorage.
// utils.ts only exposes two helpers now: clearSession (redirect) and
// roleHomePath (pure). Both are trivial and safe under jsdom.

describe("roleHomePath", () => {
  it.each([
    ["admin", "/admin"],
    ["doctor", "/doctor/dashboard"],
    ["provider", "/provider/dashboard"],
    ["patient", "/dashboard"],
  ] as const)("maps %s -> %s", (role, home) => {
    expect(roleHomePath(role)).toBe(home);
  });

  it("returns /login for unknown or null roles", () => {
    expect(roleHomePath(null)).toBe("/login");
    expect(roleHomePath(undefined)).toBe("/login");
    expect(roleHomePath("nurse")).toBe("/login");
    expect(roleHomePath("")).toBe("/login");
  });
});

describe("clearSession", () => {
  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it("redirects to /login when redirectToLogin is true (and not already there)", () => {
    const loc = {
      pathname: "/dashboard",
      href: "http://localhost/dashboard",
    } as { pathname: string; href: string };
    vi.stubGlobal("location", loc);
    clearSession(true);
    expect(loc.href).toBe("/login");
  });

  it("is a no-op when redirectToLogin is false", () => {
    const loc = {
      pathname: "/dashboard",
      href: "http://localhost/dashboard",
    } as { pathname: string; href: string };
    vi.stubGlobal("location", loc);
    clearSession(false);
    expect(loc.href).toBe("http://localhost/dashboard");
  });

  it("is a safe no-op in an SSR context (no window)", () => {
    const originalWindow = globalThis.window;
    delete (globalThis as Record<string, unknown>).window;
    expect(() => clearSession(true)).not.toThrow();
    (globalThis as Record<string, unknown>).window = originalWindow;
  });
});
