import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import { stubLocation } from "../helpers/location";

// Control the unified session endpoint directly; the guard's logic is
// a pure function of its response + requiredRole.
vi.mock("@/lib/api", () => ({
  getSession: vi.fn(),
}));
import { getSession } from "@/lib/api";

import { RouteGuard } from "@/components/guards/RouteGuard";

function mockSession(value: unknown) {
  (getSession as unknown as ReturnType<typeof vi.fn>).mockResolvedValue(value);
}

const SESSION = (over: Record<string, unknown> = {}) => ({
  user: {
    id: 1,
    username: "u",
    email: "u@h.com",
    role: "patient",
    status: "active",
    ...over,
  },
  access_expires_at: "2026-11-18T00:00:00Z",
});

beforeEach(() => {
  window.history.replaceState(null, "", "/");
  vi.mocked(getSession).mockClear();
});

afterEach(() => vi.unstubAllGlobals());

describe("RouteGuard matrix", () => {
  it("no session -> redirects to /login and shows nothing but the loading state", async () => {
    mockSession(null);
    const loc = stubLocation("/dashboard");

    render(
      <RouteGuard requiredRole="patient">
        <div data-testid="content">secret</div>
      </RouteGuard>
    );

    await waitFor(() => expect(loc.href).toBe("/login"));
    expect(screen.queryByTestId("content")).toBeNull();
    vi.unstubAllGlobals();
  });

  it("wrong role -> 403 view with a link to the user's own home (not /login)", async () => {
    mockSession(SESSION({ role: "patient" }));
    const loc = stubLocation("/admin");

    render(
      <RouteGuard requiredRole="admin">
        <div data-testid="content">secret</div>
      </RouteGuard>
    );

    // The 403 view is shown instead of the children; the escape hatch is the
    // patient's home path.
    expect(await screen.findByText("Access denied")).toBeInTheDocument();
    expect(screen.queryByTestId("content")).toBeNull();
    const home = screen.getByRole("link", { name: /go to my home/i });
    expect(home).toHaveAttribute("href", "/dashboard");
    vi.unstubAllGlobals();
  });

  it("suspended account -> blocked view, no children", async () => {
    mockSession(SESSION({ status: "suspended" }));
    const loc = stubLocation("/admin");

    render(
      <RouteGuard requiredRole="admin">
        <div data-testid="content">secret</div>
      </RouteGuard>
    );

    expect(await screen.findByText(/Account suspended/i)).toBeInTheDocument();
    expect(screen.queryByTestId("content")).toBeNull();
    vi.unstubAllGlobals();
  });

  it("pending account -> blocked view naming the status", async () => {
    mockSession(SESSION({ status: "pending", role: "doctor" }));
    const loc = stubLocation("/doctor/dashboard");

    render(
      <RouteGuard requiredRole="doctor">
        <div data-testid="content">secret</div>
      </RouteGuard>
    );

    expect(await screen.findByText(/Account pending/i)).toBeInTheDocument();
    expect(screen.queryByTestId("content")).toBeNull();
    vi.unstubAllGlobals();
  });

  it("active + matching role -> renders children", async () => {
    mockSession(SESSION());
    const loc = stubLocation("/dashboard");

    render(
      <RouteGuard requiredRole="patient">
        <div data-testid="content">secret</div>
      </RouteGuard>
    );

    expect(await screen.findByTestId("content")).toHaveTextContent("secret");
    vi.unstubAllGlobals();
  });

  it("accepts a role array (multiple allowed roles)", async () => {
    mockSession(SESSION({ role: "doctor" }));
    const loc = stubLocation("/doctor/dashboard");

    render(
      <RouteGuard requiredRole={["doctor", "provider"]}>
        <div data-testid="content">secret</div>
      </RouteGuard>
    );

    expect(await screen.findByTestId("content")).toHaveTextContent("secret");
    vi.unstubAllGlobals();
  });

  it("unknown status value blocks instead of rendering (deny by default)", async () => {
    mockSession(SESSION({ status: "weird" }));
    const loc = stubLocation("/dashboard");

    render(
      <RouteGuard requiredRole="patient">
        <div data-testid="content">secret</div>
      </RouteGuard>
    );

    // Only 'active' may pass; anything else renders the blocked state.
    expect(await screen.findByText(/Account weird/i)).toBeInTheDocument();
    expect(screen.queryByTestId("content")).toBeNull();
    vi.unstubAllGlobals();
  });
});
