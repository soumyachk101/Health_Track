import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import {
  API_URL,
  ApiError,
  apiFetch,
  apiLogin,
  apiProfile,
  getApiUrl,
  logout,
  refreshSession,
} from "@/lib/api";
import { stubLocation } from "../helpers/location";

// jsdom has a real localStorage; the cookie client no longer touches it, but
// location must be stubbed to observe the logout redirect.
beforeEach(() => {
  window.localStorage.clear();
  window.history.replaceState(null, "", "/");
});

afterEach(() => {
  vi.unstubAllGlobals();
  vi.restoreAllMocks();
});

function jsonResponse(body: unknown, status: number, ok?: boolean) {
  return {
    ok: ok ?? (status >= 200 && status < 300),
    status,
    json: async () => body,
  };
}

function stubFetch() {
  const mock = vi.fn();
  vi.stubGlobal("fetch", mock);
  return mock;
}

describe("lib/api apiFetch — cookie auth", () => {
  it("sends credentials: include and parses JSON (no Authorization header)", async () => {
    const mock = stubFetch();
    mock.mockResolvedValue(jsonResponse({ a: 1 }, 200));

    const data = await apiFetch("/x");

    expect(data).toEqual({ a: 1 });
    expect(mock).toHaveBeenCalledTimes(1);
    const [url, init] = mock.mock.calls[0] as unknown as [string, RequestInit];
    expect(url).toBe(getApiUrl("/x"));
    expect(url).toBe(`${API_URL}/x`);
    expect(init.credentials).toBe("include");
    expect((init.headers as Record<string, string>)).not.toHaveProperty("Authorization");
  });

  it("on 401: refreshes once, retries the same request, and resolves when the retry succeeds", async () => {
    const mock = stubFetch();
    mock
      .mockResolvedValueOnce(jsonResponse({ error: "token expired" }, 401, false)) // original
      .mockResolvedValueOnce(jsonResponse({ success: true }, 200)) // POST /api/v1/auth/refresh
      .mockResolvedValueOnce(jsonResponse({ a: 1 }, 200)); // the retry

    const data = await apiFetch<{ a: number }>("/x");

    expect(data).toEqual({ a: 1 });
    expect(mock).toHaveBeenCalledTimes(3);
    const refreshCall = mock.mock.calls[1] as unknown as [string, RequestInit];
    expect(refreshCall[0]).toBe(`${API_URL}/api/v1/auth/refresh`);
    expect(refreshCall[1].method).toBe("POST");
    expect(refreshCall[1].credentials).toBe("include");
    const retryCall = mock.mock.calls[2] as unknown as [string, RequestInit];
    expect(retryCall[0]).toBe(`${API_URL}/x`);
  });

  it("on 401 when refresh fails: redirects to /login (off /login) and throws", async () => {
    const mock = stubFetch();
    mock.mockResolvedValue(jsonResponse({ success: true }, 200)); // default (logout's fetch)
    mock
      .mockResolvedValueOnce(jsonResponse({ error: "token expired" }, 401, false))
      .mockResolvedValueOnce(jsonResponse({ success: false, error: "unauthenticated" }, 401, false)); // refresh fails
    const loc = stubLocation("/dashboard");

    const err = await apiFetch("/x").catch((e) => e);
    expect(err).toBeInstanceOf(ApiError);
    expect((err as ApiError).status).toBe(401);
    // logout() was invoked: it fired the best-effort revoke call and navigated.
    expect(loc.href).toBe("/login");
  });

  it("on 401 when the retry still 401s: redirects and throws", async () => {
    const mock = stubFetch();
    // Default for logout()'s best-effort revoke fetch (4th call onward).
    mock.mockResolvedValue(jsonResponse({ success: true }, 200));
    mock
      .mockResolvedValueOnce(jsonResponse({}, 401, false))
      .mockResolvedValueOnce(jsonResponse({ success: true }, 200)) // refresh ok
      .mockResolvedValueOnce(jsonResponse({ error: "still no" }, 401, false)); // retry 401s again
    const loc = stubLocation("/dashboard");

    const err = await apiFetch("/x").catch((e) => e);
    expect(err).toBeInstanceOf(ApiError);
    expect((err as ApiError).status).toBe(401);
    expect(loc.href).toBe("/login");
  });

  it("a 401 on an unauthenticated entry-point call (apiLogin) does NOT refresh or redirect", async () => {
    const mock = stubFetch();
    mock.mockResolvedValue(jsonResponse({ success: false, error: "Invalid credentials" }, 401, false));
    const loc = stubLocation("/login");

    await expect(apiLogin("u", "p")).rejects.toThrow(ApiError);
    // One call total: no refresh, no retry.
    expect(mock).toHaveBeenCalledTimes(1);
    expect((mock.mock.calls[0] as unknown as [string, RequestInit])[0]).toBe(`${API_URL}/api/v1/auth/login`);
    expect(loc.href).toBe("http://localhost/login"); // no redirect
  });

  it("throws ApiError with the body message on 2xx {success:false}... but only via !res.ok paths", async () => {
    // The Go backend signals failure via non-2xx status; a 200 is returned
    // as-is. Assert a 500 surfaces as an ApiError with the server message.
    const mock = stubFetch();
    mock.mockResolvedValue(jsonResponse({ error: "server exploded" }, 500, false));

    const err = await apiFetch("/x", { protected: false }).catch((e) => e);
    expect(err).toBeInstanceOf(ApiError);
    expect((err as ApiError).status).toBe(500);
    expect((err as ApiError).message).toBe("server exploded");
  });

  it("on 403 (wrong role) it does not refresh; throws ApiError with the body message", async () => {
    const mock = stubFetch();
    mock.mockResolvedValue(jsonResponse({ error: "forbidden" }, 403, false));

    const err = await apiFetch("/x").catch((e) => e);
    expect(err).toBeInstanceOf(ApiError);
    expect((err as ApiError).status).toBe(403);
    expect(mock).toHaveBeenCalledTimes(1); // no refresh on 403
  });
});

describe("lib/api refreshSession — single-flight", () => {
  it("coalesces concurrent refreshes into one POST /refresh", async () => {
    const mock = stubFetch();
    type PartialRes = ReturnType<typeof jsonResponse>;
    let resolveRefresh: (v: PartialRes) => void;
    mock.mockImplementation(async (url: string) => {
      if (String(url).endsWith("/api/v1/auth/refresh")) {
        return new Promise<PartialRes>((r) => {
          resolveRefresh = r;
        });
      }
      return jsonResponse({ ok: true }, 200);
    });

    const p1 = refreshSession();
    const p2 = refreshSession();
    const p3 = refreshSession();

    resolveRefresh!(jsonResponse({ success: true }, 200));
    await Promise.all([p1, p2, p3]);

    const refreshCalls = mock.mock.calls.filter(
      (c) => String(c[0]).endsWith("/api/v1/auth/refresh")
    );
    expect(refreshCalls).toHaveLength(1);
    expect(await p1).toBe(true);
    expect(await p2).toBe(true);
    expect(await p3).toBe(true);
  });

  it("resolves false on a network failure (does not throw)", async () => {
    stubFetch().mockRejectedValue(new Error("offline"));
    await expect(refreshSession()).resolves.toBe(false);
  });
});

describe("lib/api logout", () => {
  it("fires a best-effort POST /api/v1/auth/logout and redirects to /login", async () => {
    const mock = stubFetch();
    mock.mockResolvedValue(jsonResponse({ success: true }, 200));
    const loc = stubLocation("/dashboard");

    logout();
    // The revoke fetch is fire-and-forget; flush microtasks.
    await new Promise((r) => setTimeout(r, 0));

    const [url, init] = mock.mock.calls[0] as unknown as [string, RequestInit];
    expect(url).toBe(`${API_URL}/api/v1/auth/logout`);
    expect(init.method).toBe("POST");
    expect(init.credentials).toBe("include");
    expect(loc.href).toBe("/login");
  });

  it("does not redirect when already on the target path", async () => {
    stubFetch().mockResolvedValue(jsonResponse({ success: true }, 200));
    const loc = stubLocation("/login");
    logout();
    await new Promise((r) => setTimeout(r, 0));
    expect(loc.href).toBe("http://localhost/login");
  });

  it("redirects even when the revoke call fails (best-effort)", async () => {
    stubFetch().mockRejectedValue(new Error("offline"));
    const loc = stubLocation("/admin");
    logout();
    await new Promise((r) => setTimeout(r, 0));
    expect(loc.href).toBe("/login");
  });
});

describe("lib/api typed auth helpers", () => {
  it("apiLogin posts {username,password} to /api/v1/auth/login", async () => {
    const mock = stubFetch();
    mock.mockResolvedValue(jsonResponse({ success: true, user: { role: "patient" } }, 200));

    await apiLogin("u", "p");

    const [url, init] = mock.mock.calls[0] as unknown as [string, RequestInit];
    expect(url).toBe(`${API_URL}/api/v1/auth/login`);
    expect(init.method).toBe("POST");
    expect(JSON.parse(init.body as string)).toEqual({ username: "u", password: "p" });
    expect((init.headers as Record<string, string>)).not.toHaveProperty("Authorization");
    expect(init.credentials).toBe("include");
  });

  it("apiProfile GETs /api/profile with credentials: include", async () => {
    const mock = stubFetch();
    mock.mockResolvedValue(jsonResponse({ user: {}, messages: [] }, 200));

    await apiProfile();

    const [url, init] = mock.mock.calls[0] as unknown as [string, RequestInit];
    expect(url).toBe(`${API_URL}/api/profile`);
    expect(init.method).toBe("GET");
    expect(init.body).toBeUndefined();
    expect(init.credentials).toBe("include");
  });
});
