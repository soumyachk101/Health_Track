import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { roleOfSupaUser } from "@/lib/supabase";
import {
  TOKEN_STORAGE_KEY,
  getStoredToken,
  getSessionRole,
  syncSessionFromSupabase,
  clearSupaSession,
  getSupabaseSessionInfo,
} from "@/lib/utils";

function clearSessionKeys() {
  window.localStorage.clear();
}

function setSupaEnv() {
  vi.stubEnv("NEXT_PUBLIC_SUPABASE_URL", "https://sbtest.supabase.co");
  vi.stubEnv("NEXT_PUBLIC_SUPABASE_ANON_KEY", "anon-test-key");
}

function unsetSupaEnv() {
  vi.stubEnv("NEXT_PUBLIC_SUPABASE_URL", "");
  vi.stubEnv("NEXT_PUBLIC_SUPABASE_ANON_KEY", "");
}

// The Supabase client is a module-level singleton. Resetting the module
// registry before each client-dependent test gives a fresh (uncached)
// singleton so tests never hit the network and env branches are deterministic.
async function freshSupabase() {
  vi.resetModules();
  return await import("@/lib/supabase");
}

describe("lib/supabase getSupabase null-guard", () => {
  afterEach(() => vi.unstubAllEnvs());

  it("returns null (does not throw) when the env vars are unset", async () => {
    unsetSupaEnv();
    const sb = await freshSupabase();
    expect(sb.getSupabase()).toBeNull();
  });

  it("creates a client (with .auth surface) when both env vars are set", async () => {
    setSupaEnv();
    const sb = await freshSupabase();
    const client = sb.getSupabase();
    expect(client).not.toBeNull();
    expect(typeof client.auth.signInWithPassword).toBe("function");
    expect(typeof client.auth.onAuthStateChange).toBe("function");
  });

  it("supaSignIn rejects with SupabaseNotConfiguredError when unset (no network)", async () => {
    unsetSupaEnv();
    const sb = await freshSupabase();
    let caught: unknown;
    try {
      await sb.supaSignIn("a@b.com", "pw");
    } catch (e) {
      caught = e;
    }
    // A FRESH module instance's error class is not `instanceof` the top-level
    // import, so assert on the error's name + type instead of class identity.
    expect(caught).toBeTruthy();
    expect((caught as Error).name).toBe("SupabaseNotConfiguredError");
  });

  it("supaSignOut is a no-op that does not throw when unset", async () => {
    unsetSupaEnv();
    const sb = await freshSupabase();
    await expect(sb.supaSignOut()).resolves.toBeUndefined();
  });
});

describe("lib/supabase roleOfSupaUser", () => {
  it("reads role from user_metadata when present", () => {
    expect(roleOfSupaUser({ id: "1", email: "a@b.com", user_metadata: { role: "doctor" } })).toBe("doctor");
  });

  it("returns null when role is absent, empty, or non-string", () => {
    expect(roleOfSupaUser({ id: "1", email: "a@b.com" })).toBeNull();
    expect(roleOfSupaUser({ id: "1", email: "a@b.com", user_metadata: {} })).toBeNull();
    expect(roleOfSupaUser({ id: "1", email: "a@b.com", user_metadata: { role: "" } })).toBeNull();
    expect(roleOfSupaUser({ id: "1", email: "a@b.com", user_metadata: { role: 42 } })).toBeNull();
    expect(roleOfSupaUser(null)).toBeNull();
    expect(roleOfSupaUser(undefined)).toBeNull();
  });
});

describe("lib/utils Supabase session bridge", () => {
  beforeEach(clearSessionKeys);

  it("syncSessionFromSupabase writes the canonical + legacy keys with the access token and role", () => {
    syncSessionFromSupabase("supa-access-token", "provider");

    expect(getStoredToken()).toBe("supa-access-token");
    expect(window.localStorage.getItem(TOKEN_STORAGE_KEY)).toBe("supa-access-token");
    expect(window.localStorage.getItem("token")).toBe("supa-access-token");
    expect(window.localStorage.getItem("ht_role")).toBe("provider");
    expect(JSON.parse(window.localStorage.getItem("user")!)).toEqual({ role: "provider" });
    expect(getSessionRole()).toBe("provider");
  });

  it("syncSessionFromSupabase with a null role writes an empty role string", () => {
    syncSessionFromSupabase("tok", null);
    expect(getStoredToken()).toBe("tok");
    expect(window.localStorage.getItem("ht_role")).toBe("");
  });

  it("clearSupaSession drops the Supabase-managed session keys", () => {
    syncSessionFromSupabase("tok", "patient");
    clearSupaSession();

    expect(window.localStorage.getItem(TOKEN_STORAGE_KEY)).toBeNull();
    expect(window.localStorage.getItem("ht_role")).toBeNull();
    expect(window.localStorage.getItem("token")).toBeNull();
    expect(window.localStorage.getItem("user")).toBeNull();
  });

  it("getSupabaseSessionInfo reads the stored token + role for client boot", () => {
    syncSessionFromSupabase("boot-token", "doctor");
    expect(getSupabaseSessionInfo()).toEqual({ accessToken: "boot-token", role: "doctor" });

    clearSupaSession();
    expect(getSupabaseSessionInfo()).toEqual({ accessToken: null, role: null });
  });

  it("getSupabaseSessionInfo is safe when there is nothing stored", () => {
    expect(getSupabaseSessionInfo()).toEqual({ accessToken: null, role: null });
  });
});
