import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";

// The page calls useSearchParams, so mock next/navigation.
vi.mock("next/navigation", () => ({
  useRouter: vi.fn(() => ({ push: vi.fn() })),
  useSearchParams: vi.fn(() => new URLSearchParams()),
}));

// Mock the central cookie client: apiLogin is a controllable vi.fn; the rest
// of the module (ApiError, roleHomePath re-export, ...) runs for real.
vi.mock("@/lib/api", async () => {
  const actual = await vi.importActual<typeof import("@/lib/api")>("@/lib/api");
  return {
    ...actual,
    apiLogin: vi.fn(),
    apiGoogleLogin: vi.fn(),
  };
});

import { useRouter } from "next/navigation";
import Login from "@/app/(auth)/login/page";
import { apiLogin, ApiError } from "@/lib/api";

function mockLogin(value: unknown) {
  (apiLogin as unknown as ReturnType<typeof vi.fn>).mockResolvedValue(value);
}

beforeEach(() => {
  window.localStorage.clear();
  window.history.replaceState(null, "", "/login");
  vi.mocked(apiLogin).mockClear();
});

afterEach(() => vi.unstubAllGlobals());

describe("login page submit flow (Pure Go cookie auth)", () => {
  it("signs in via POST /api/v1/auth/login and routes to the role home", async () => {
    mockLogin({ success: true, user: { role: "patient", status: "active" } });
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
      expect(apiLogin).toHaveBeenCalledWith("doc@h.com", "pass123");
    });
    // patient role resolves to the dashboard home.
    expect(push).toHaveBeenCalledWith("/dashboard");
  });

  it("surfaces a wrong-credentials 401 as a friendly error (no redirect)", async () => {
    (apiLogin as unknown as ReturnType<typeof vi.fn>).mockRejectedValue(
      new ApiError("Invalid credentials", 401, { success: false })
    );
    const push = vi.fn();
    (useRouter as unknown as ReturnType<typeof vi.fn>).mockReturnValue({ push });

    const { baseElement } = render(<Login />);
    const user = userEvent.setup();
    await user.type(screen.getByLabelText("Identify Yourself"), "doc@h.com");
    await user.type(screen.getByLabelText("Security Key"), "wrong");
    await user.click(screen.getByRole("button", { name: /authenticate/i }));

    await waitFor(() => {
      expect(baseElement).toHaveTextContent(/incorrect username or password/i);
    });
    // No routing on the error path.
    expect(push).not.toHaveBeenCalled();
  });

  it("falls back to /login routing when the server omits a role", async () => {
    // No user in the body -> the page defaults to 'patient' home.
    mockLogin({ success: true });
    const push = vi.fn();
    (useRouter as unknown as ReturnType<typeof vi.fn>).mockReturnValue({ push });

    render(<Login />);
    const user = userEvent.setup();
    await user.type(screen.getByLabelText("Identify Yourself"), "doc@h.com");
    await user.type(screen.getByLabelText("Security Key"), "pass123");
    await user.click(screen.getByRole("button", { name: /authenticate/i }));

    await waitFor(() => expect(apiLogin).toHaveBeenCalled());
    expect(push).toHaveBeenCalledWith("/dashboard");
  });

  it("blocks an empty username with a field error", async () => {
    render(<Login />);
    const user = userEvent.setup();
    await user.type(screen.getByLabelText("Security Key"), "pass123");
    await user.click(screen.getByRole("button", { name: /authenticate/i }));

    expect(await screen.findByText("Please enter your username or email.")).toBeInTheDocument();
    expect(apiLogin).not.toHaveBeenCalled();
  });
});
