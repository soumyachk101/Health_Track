import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import {
  API_URL,
  ApiError,
  apiFetch,
  apiLogin,
  apiProfile,
  apiResetPassword,
  getApiUrl,
} from "@/lib/api";
import { stubLocation } from "../helpers/location";

// jsdom's localStorage is real; seed/clear the session there so the real
// getStoredToken/clearSession behave as they would in the browser.
beforeEach(() => {
  window.localStorage.clear();
  window.history.replaceState(null, "", "/");
});

function jsonResponse(body: unknown, status: number, ok?: boolean) {
  return {
    ok: ok ?? (status >= 200 && status < 300),
    status,
    json: async () => body,
  };
}

function fetchMock() {
  return vi.fn().mockResolvedValue(jsonResponse({ a: 1 }, 200));
}

function stubFetch(): ReturnType<typeof fetchMock> {
  const mock = fetchMock();
  vi.stubGlobal("fetch", mock);
  return mock;
}

describe("lib/api apiFetch", () => {
  afterEach(() => vi.unstubAllGlobals());

  it("returns parsed JSON and calls fetch with getApiUrl(path), no Bearer when auth:false", async () => {
    const fetchMock = stubFetch();

    const data = await apiFetch("/x", { method: "GET", auth: false });

    expect(data).toEqual({ a: 1 });
    expect(fetchMock).toHaveBeenCalledTimes(1);
    expect(fetchMock.mock.calls[0][0]).toBe(getApiUrl("/x"));
    expect(fetchMock.mock.calls[0][0]).toBe(`${API_URL}/x`);
    expect(fetchMock.mock.calls[0][1].headers).not.toHaveProperty("Authorization");
  });

  it("attaches the stored Bearer token when auth:true", async () => {
    const fetchMock = stubFetch();
    window.localStorage.setItem("ht_token", "tok");

    await apiFetch("/x", { auth: true });

    expect(fetchMock.mock.calls[0][1].headers.Authorization).toBe("Bearer tok");
  });

  it("on 401 with auth:true: clears the session, redirects off /login, throws ApiError", async () => {
    const fetchMock = stubFetch();
    fetchMock.mockResolvedValue(jsonResponse({ error: "expired" }, 401, false));
    window.localStorage.setItem("ht_token", "tok");
    window.localStorage.setItem("ht_role", "patient");
    const loc = stubLocation("/dashboard");

    try {
      const err = await apiFetch("/x", { auth: true }).catch((e) => e);
      expect(err).toBeInstanceOf(ApiError);
      expect((err as ApiError).status).toBe(401);
      expect((err as ApiError).message).toBe("expired");
      expect(window.localStorage.getItem("ht_token")).toBeNull();
      expect(window.localStorage.getItem("ht_role")).toBeNull();
      expect(loc.href).toBe("/login");
    } finally {
      vi.unstubAllGlobals();
      vi.restoreAllMocks();
    }
  });

  it("on 401 does not redirect when already on /login", async () => {
    const fetchMock = stubFetch();
    fetchMock.mockResolvedValue(jsonResponse({ error: "expired" }, 401, false));
    window.localStorage.setItem("ht_token", "tok");
    const loc = stubLocation("/login");

    await expect(apiFetch("/x", { auth: true })).rejects.toThrow(ApiError);
    // Already on /login -> the redirect is skipped, href stays put.
    expect(loc.href).toBe("http://localhost/login");
    vi.unstubAllGlobals();
    vi.restoreAllMocks();
  });

  it("on 500: throws ApiError with status 500", async () => {
    const fetchMock = stubFetch();
    fetchMock.mockResolvedValue(jsonResponse({ error: "server exploded" }, 500, false));

    const err = await apiFetch("/x", { auth: false }).catch((e) => e);
    expect(err).toBeInstanceOf(ApiError);
    expect((err as ApiError).status).toBe(500);
  });

  it("a 200 {success:false} body still throws ApiError carrying the error message", async () => {
    const fetchMock = stubFetch();
    fetchMock.mockResolvedValue(jsonResponse({ success: false, error: "boom" }, 200, true));

    const err = await apiFetch("/x", { auth: false }).catch((e) => e);
    expect(err).toBeInstanceOf(ApiError);
    expect((err as ApiError).status).toBe(200);
    expect((err as ApiError).message).toBe("boom");
  });

  it("on a 401 the session is not cleared when auth:false", async () => {
    const fetchMock = stubFetch();
    fetchMock.mockResolvedValue(jsonResponse({ error: "expired" }, 401, false));
    window.localStorage.setItem("ht_token", "tok");

    await expect(apiFetch("/x", { auth: false })).rejects.toThrow(ApiError);
    expect(window.localStorage.getItem("ht_token")).toBe("tok");
  });
});

describe("lib/api typed helpers", () => {
  afterEach(() => vi.unstubAllGlobals());

  it("apiLogin posts to /accounts/api/login without auth", async () => {
    const fetchMock = stubFetch();
    fetchMock.mockResolvedValue(jsonResponse({ success: true, token: "t", user: { role: "patient" } }, 200));

    await apiLogin("u", "p");

    const [url, init] = fetchMock.mock.calls[0] as unknown as [string, RequestInit];
    expect(url).toBe(`${API_URL}/accounts/api/login`);
    expect(init.method).toBe("POST");
    expect(JSON.parse(init.body as string)).toEqual({ username: "u", password: "p" });
    expect(init.headers).not.toHaveProperty("Authorization");
  });

  it("apiResetPassword posts {email, otp, new_password}", async () => {
    const fetchMock = stubFetch();

    await apiResetPassword("e", "otp", "np");

    const [url, init] = fetchMock.mock.calls[0] as unknown as [string, RequestInit];
    expect(url).toBe(`${API_URL}/accounts/api/reset-password`);
    expect(JSON.parse(init.body as string)).toEqual({ email: "e", otp: "otp", new_password: "np" });
  });

  it("apiProfile does a GET /api/profile with auth", async () => {
    const fetchMock = stubFetch();

    await apiProfile();

    const [url, init] = fetchMock.mock.calls[0] as unknown as [string, RequestInit];
    expect(url).toBe(`${API_URL}/api/profile`);
    expect(init.method).toBe("GET");
    expect(init.body).toBeUndefined();
  });
});
