import { afterAll, beforeEach, describe, expect, it } from "vitest";
import {
  ROLE_STORAGE_KEY,
  TOKEN_STORAGE_KEY,
  clearSession,
  getStoredToken,
  getSessionRole,
  setSession,
} from "@/lib/utils";

const ALL_KEYS = [TOKEN_STORAGE_KEY, ROLE_STORAGE_KEY, "token", "user"];

function clearAllKeys() {
  for (const key of ALL_KEYS) window.localStorage.removeItem(key);
}

describe("lib/utils session helpers", () => {
  beforeEach(() => clearAllKeys());

  it("setSession writes the canonical and legacy keys", () => {
    setSession("tok", "patient", { id: 1 });

    expect(getStoredToken()).toBe("tok");
    expect(window.localStorage.getItem(TOKEN_STORAGE_KEY)).toBe("tok");
    expect(window.localStorage.getItem("token")).toBe("tok");
    expect(window.localStorage.getItem(ROLE_STORAGE_KEY)).toBe("patient");
    expect(JSON.parse(window.localStorage.getItem("user")!)).toEqual({ id: 1, role: "patient" });
  });

  it("clearSession removes the canonical and legacy keys", () => {
    setSession("tok", "patient", { id: 1 });
    clearSession();

    for (const key of ALL_KEYS) expect(window.localStorage.getItem(key)).toBeNull();
  });

  it("getStoredToken falls back to the legacy 'token' key", () => {
    window.localStorage.setItem("token", "legacy-tok");

    expect(getStoredToken()).toBe("legacy-tok");
  });

  it("getSessionRole falls back to the legacy user JSON's role", () => {
    window.localStorage.setItem("user", JSON.stringify({ role: "admin" }));

    expect(getSessionRole()).toBe("admin");
  });

  it("getSessionRole returns null for malformed legacy user JSON", () => {
    window.localStorage.setItem("user", "not json");

    expect(getSessionRole()).toBeNull();
  });

  describe("when window is undefined (SSR)", () => {
    const originalWindow = globalThis.window;

    beforeEach(() => {
      // jsdom defines window on globalThis; delete it to simulate Node SSR.
      delete (globalThis as Record<string, unknown>).window;
    });

    it("every helper is a safe no-op", () => {
      expect(() => {
        setSession("tok", "patient");
        clearSession(true);
      }).not.toThrow();
      expect(getStoredToken()).toBeNull();
      expect(getSessionRole()).toBeNull();
    });

    afterAll(() => {
      (globalThis as Record<string, unknown>).window = originalWindow;
    });
  });
});
