import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import { roleHomePath } from "@/lib/utils";

// Mock the unified session endpoint; AuthProvider is now driven entirely by it.
vi.mock("@/lib/api", () => ({
  getSession: vi.fn(),
  logout: vi.fn(),
}));
import { getSession, logout } from "@/lib/api";

import Providers, { RequireAuth, useAuth } from "@/components/Providers";

function mockSession(value: unknown) {
  (getSession as unknown as ReturnType<typeof vi.fn>).mockResolvedValue(value);
}

function sessionFor(role: string, status = "active") {
  return {
    user: {
      id: 1,
      username: "u",
      email: "u@h.com",
      role,
      status,
    },
    access_expires_at: "2026-11-18T00:00:00Z",
  };
}

function AuthProbe() {
  const { token, role, status, isAuthenticated, ready } = useAuth();
  return (
    <div data-testid="probe">
      {JSON.stringify({ token, role, status, isAuthenticated, ready })}
    </div>
  );
}

beforeEach(() => {
  window.history.replaceState(null, "", "/");
  vi.mocked(getSession).mockClear();
  vi.mocked(logout).mockClear();
});

afterEach(() => vi.unstubAllGlobals());

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

describe("AuthProvider hydration from the session endpoint", () => {
  it("with no session: ready becomes true, unauthenticated, null role/status", async () => {
    mockSession(null);
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
      expect(probe.status).toBeNull();
      expect(probe.token).toBeNull();
    });
  });

  it("with a live session: hydrates role + status and marks authenticated", async () => {
    mockSession(sessionFor("doctor"));
    render(
      <Providers>
        <AuthProbe />
      </Providers>
    );

    await waitFor(() => {
      const probe = JSON.parse(screen.getByTestId("probe").textContent!);
      expect(probe.ready).toBe(true);
      expect(probe.isAuthenticated).toBe(true);
      expect(probe.role).toBe("doctor");
      expect(probe.status).toBe("active");
    });
  });

  it("logout() calls the api logout helper", async () => {
    mockSession(sessionFor("patient"));
    render(
      <Providers>
        <LogoutProbe />
      </Providers>
    );
    await waitFor(() => {
      const probe = JSON.parse(screen.getByTestId("probe").textContent!);
      expect(probe.ready).toBe(true);
    });
    screen.getByRole("button", { name: /log out/i }).click();
    await waitFor(() => expect(logout).toHaveBeenCalledTimes(1));
  });
});

function LogoutProbe() {
  const { logout: doLogout, ready } = useAuth();
  return (
    <>
      <div data-testid="probe">
        {JSON.stringify({ ready })}
      </div>
      <button onClick={doLogout}>Log out</button>
    </>
  );
}

describe("RequireAuth", () => {
  it("renders nothing for a session with no role grant", async () => {
    mockSession(null);
    render(
      <Providers>
        <RequireAuth>
          <div data-testid="content">secret</div>
        </RequireAuth>
      </Providers>
    );
    // Wait out hydration before asserting.
    await waitFor(() => expect(getSession).toHaveBeenCalledTimes(1));
    expect(screen.queryByTestId("content")).toBeNull();
  });

  it("renders children when the session role matches", async () => {
    mockSession(sessionFor("patient"));
    render(
      <Providers>
        <RequireAuth requireRole="patient">
          <div data-testid="content">secret</div>
        </RequireAuth>
      </Providers>
    );
    expect(await screen.findByTestId("content")).toHaveTextContent("secret");
  });

  it("renders nothing when the role does not match", async () => {
    mockSession(sessionFor("patient"));
    render(
      <Providers>
        <RequireAuth requireRole="admin">
          <div data-testid="content">secret</div>
        </RequireAuth>
      </Providers>
    );
    await waitFor(() => expect(getSession).toHaveBeenCalledTimes(1));
    expect(screen.queryByTestId("content")).toBeNull();
  });

  it("renders nothing for a non-active status even when the role matches", async () => {
    mockSession(sessionFor("admin", "suspended"));
    render(
      <Providers>
        <RequireAuth requireRole="admin">
          <div data-testid="content">secret</div>
        </RequireAuth>
      </Providers>
    );
    await waitFor(() => expect(getSession).toHaveBeenCalledTimes(1));
    expect(screen.queryByTestId("content")).toBeNull();
  });
});
