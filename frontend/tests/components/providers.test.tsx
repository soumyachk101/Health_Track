import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import { stubLocation } from "../helpers/location";

// Mock the whole @/lib/api module so apiProfile is a controllable vi.fn;
// other modules (utils, Providers, GoogleOAuthProvider) run for real.
vi.mock("@/lib/api", () => ({
  apiProfile: vi.fn(),
}));
// vi.mock factories are hoisted above imports, so this binding is a mock.
import { apiProfile } from "@/lib/api";

import Providers, { RequireAuth, roleHomePath, useAuth } from "@/components/Providers";

function AuthProbe() {
  const { token, profile, role, isAuthenticated, ready } = useAuth();
  return (
    <div data-testid="probe">
      {JSON.stringify({ token, role, profile: profile ? profile.user : null, isAuthenticated, ready })}
    </div>
  );
}

function mockProfile(value: unknown) {
  (apiProfile as unknown as ReturnType<typeof vi.fn>).mockResolvedValue(value);
}

beforeEach(() => {
  window.localStorage.clear();
  window.history.replaceState(null, "", "/");
  vi.mocked(apiProfile).mockClear();
});

afterEach(() => vi.restoreAllMocks());

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
  });
});

describe("AuthProvider hydration (no module-mock interplay)", () => {
  it("with no stored token: ready becomes true, unauthenticated, null role", async () => {
    render(
      <Providers>
        <AuthProbe />
      </Providers>
    );

    await waitFor(() => {
      const probe = JSON.parse(screen.getByTestId("probe").textContent!);
      expect(probe.ready).toBe(true);
      expect(probe.isAuthenticated).toBe(false);
      expect(probe.role).toBeNull();
      expect(probe.token).toBeNull();
    });
    expect(apiProfile).not.toHaveBeenCalled();
  });

  it("with a stored token: hydrates role and profile from apiProfile", async () => {
    mockProfile({ user: { role: "patient" }, messages: [] });
    window.localStorage.setItem("ht_token", "tok");
    window.localStorage.setItem("ht_role", "patient");

    render(
      <Providers>
        <AuthProbe />
      </Providers>
    );

    await waitFor(() => {
      const probe = JSON.parse(screen.getByTestId("probe").textContent!);
      expect(probe.ready).toBe(true);
      expect(probe.isAuthenticated).toBe(true);
      expect(probe.role).toBe("patient");
      expect(probe.profile).toEqual({ role: "patient" });
    });
    expect(apiProfile).toHaveBeenCalledTimes(1);
  });
});

describe("RequireAuth", () => {
  it("renders nothing when there is no token", async () => {
    const loc = stubLocation("/");

    render(
      <Providers>
        <RequireAuth>
          <div data-testid="content">secret</div>
        </RequireAuth>
      </Providers>
    );

    // Hydration is async (Supabase getSession), so the no-token redirect to
    // /login lands a beat later — wait for the navigation itself.
    await waitFor(() => expect(loc.href).toBe("/login"));
    expect(screen.queryByTestId("content")).toBeNull();
    vi.unstubAllGlobals();
  });

  it("renders children for a matching role", async () => {
    mockProfile({ user: { role: "patient" }, messages: [] });
    window.localStorage.setItem("ht_token", "tok");
    window.localStorage.setItem("ht_role", "patient");

    render(
      <Providers>
        <RequireAuth requireRole="patient">
          <div data-testid="content">secret</div>
        </RequireAuth>
      </Providers>
    );

    expect(await screen.findByTestId("content")).toHaveTextContent("secret");
  });

  it("renders nothing when the role does not match, and redirects to the role home", async () => {
    const loc = stubLocation("/");
    mockProfile({ user: { role: "patient" }, messages: [] });
    window.localStorage.setItem("ht_token", "tok");
    window.localStorage.setItem("ht_role", "patient");

    render(
      <Providers>
        <RequireAuth requireRole="admin">
          <div data-testid="content">secret</div>
        </RequireAuth>
      </Providers>
    );

    // The bounce only happens once the profile has hydrated (one render-cycle
    // later), so wait for the redirect itself rather than the always-null content.
    await waitFor(() => expect(loc.href).toBe("/dashboard"));
    vi.unstubAllGlobals();
  });
});
