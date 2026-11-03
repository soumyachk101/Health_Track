import { describe, expect, it, vi } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";

// The page calls useSearchParams, so mock next/navigation.
vi.mock("next/navigation", () => ({
  useRouter: vi.fn(() => ({ push: vi.fn() })),
  useSearchParams: vi.fn(() => new URLSearchParams()),
}));
// Keep framer-motion + next/link working for real.

// Mock the Supabase helpers: sign-in resolves a Supabase session whose
// user_metadata carries the app role. The rest of the module (roleOfSupaUser,
// error classes) runs for real via importActual.
vi.mock("@/lib/supabase", async () => {
  const actual = await vi.importActual<typeof import("@/lib/supabase")>("@/lib/supabase");
  return {
    ...actual,
    supaSignIn: vi.fn(),
    supaGoogleSignIn: vi.fn(),
    supaResendVerification: vi.fn(),
  };
});

// Spy on the localStorage sync so we can assert the token was persisted.
vi.mock("@/lib/utils", () => {
  const actual = vi.importActual("@/lib/utils") as Promise<typeof import("@/lib/utils")>;
  return actual.then((mod) => ({ ...mod, syncSessionFromSupabase: vi.fn() }));
});

// The role-unknown path would call apiProfile; resolve it for safety.
vi.mock("@/lib/api", () => ({
  apiProfile: vi.fn(async () => ({ user: {}, messages: [] })),
}));

import { useRouter } from "next/navigation";
import Login from "@/app/(auth)/login/page";
import { supaSignIn } from "@/lib/supabase";
import { syncSessionFromSupabase } from "@/lib/utils";

describe("login page submit flow (Supabase)", () => {
  it("signs in with Supabase, syncs the session token, and routes to the role home", async () => {
    // user_metadata.role present -> applyAuth uses it directly (no profile fetch).
    (supaSignIn as unknown as ReturnType<typeof vi.fn>).mockResolvedValue({
      accessToken: "supa-tok",
      user: { id: "u1", email: "doc@h.com", user_metadata: { role: "patient" } },
    });
    const push = vi.fn();
    (useRouter as unknown as ReturnType<typeof vi.fn>).mockReturnValue({ push });

    render(<Login />);

    const user = userEvent.setup();
    await user.type(screen.getByLabelText("Identify Yourself"), "doc@h.com");
    // The password field renders as type="password" until toggled, so query by
    // its accessible label rather than role="textbox" (which excludes it).
    await user.type(screen.getByLabelText("Security Key"), "pass123");
    await user.click(screen.getByRole("button", { name: /authenticate/i }));

    await waitFor(() => {
      expect(supaSignIn).toHaveBeenCalledWith("doc@h.com", "pass123");
      expect(syncSessionFromSupabase).toHaveBeenCalledWith("supa-tok", "patient");
    });
    // patient role resolves to the dashboard home.
    expect(push).toHaveBeenCalledWith("/dashboard");
  });

  it("surfaces an 'email not confirmed' error as the verification panel + resend", async () => {
    const { AuthError } = await import("@/lib/supabase");
    // A real AuthError instance so the page's `err instanceof AuthError`
    // branch (and its .code / message) is exercised for real.
    const err = new AuthError("Email not confirmed", 400, "email_not_confirmed");
    (supaSignIn as unknown as ReturnType<typeof vi.fn>).mockRejectedValue(err);

    const push = vi.fn();
    (useRouter as unknown as ReturnType<typeof vi.fn>).mockReturnValue({ push });

    const { baseElement } = render(<Login />);
    const user = userEvent.setup();
    await user.type(screen.getByLabelText("Identify Yourself"), "doc@h.com");
    await user.type(screen.getByLabelText("Security Key"), "pass123");
    await user.click(screen.getByRole("button", { name: /authenticate/i }));

    await waitFor(() => {
      expect(baseElement).toHaveTextContent(/requires verification/i);
    });
    // The resend button is now visible inside the amber verification panel.
    expect(screen.getByRole("button", { name: /resend link/i })).toBeInTheDocument();
    // No role routing happened on the error path.
    expect(push).not.toHaveBeenCalled();
  });

  it("redirects to the Supabase OAuth URL for the Google button", async () => {
    const { supaGoogleSignIn } = await import("@/lib/supabase");
    (supaGoogleSignIn as unknown as ReturnType<typeof vi.fn>).mockResolvedValue({
      url: "https://supabase.auth/oauth",
    });
    // jsdom's window.location.href is a non-redefinable property, so swap the
    // whole location object (as stubLocation does) to observe the navigation.
    const { stubLocation } = await import("../helpers/location");
    const loc = stubLocation("/login");

    const push = vi.fn();
    (useRouter as unknown as ReturnType<typeof vi.fn>).mockReturnValue({ push });

    render(<Login />);
    const user = userEvent.setup();
    await user.click(await screen.findByRole("button", { name: /sign in with google/i }));

    await waitFor(() => expect(loc.href).toBe("https://supabase.auth/oauth"));
    vi.unstubAllGlobals();
  });
});
