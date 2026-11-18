import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";

// The component uses the central cookie client; mock it so each test drives
// the chat/jobs responses without a network.
vi.mock("@/lib/api", () => ({
  apiFetch: vi.fn(),
  ApiError: class ApiError extends Error {},
  getSession: vi.fn(),
  logout: vi.fn(),
}));

// The AI-UI flows under test don't need voice or the real auth context. The
// real Providers hydrate from getSession (noise); a trivially-authenticated
// stub keeps the input + disclaimer areas mounted.
vi.mock("@/components/Providers", async () => {
  const { createElement } = await import("react");
  return {
    Providers: ({ children }: { children: React.ReactNode }) =>
      createElement("div", null, children),
    useAuth: () => ({ isAuthenticated: true }),
  };
});

// The backoff loop itself (2 s x 1.6, cap, budget, abort) is unit-tested in
// tests/lib/files.test.ts against the real pollJob. Here the poll is mocked so
// the component's handling of each terminal state (completed/failed) is tested
// in isolation; we only assert it is wired with an AbortSignal.
vi.mock("@/lib/files", () => ({
  pollJob: vi.fn(),
}));

import { apiFetch } from "@/lib/api";
import { pollJob } from "@/lib/files";
import Chatbot from "@/components/Chatbot";

const fetchMock = vi.mocked(apiFetch);
const pollMock = vi.mocked(pollJob);

// apiFetch returns the parsed JSON body (see lib/api: `data as T`), so the mock
// must resolve to the body itself — not a Response-like wrapper.
function jobResponse(body: unknown) {
  return body;
}
const originalScrollIntoView = Element.prototype.scrollIntoView;

// jsdom lacks the browser APIs the component touches on mount. Stub them so
// `render(<Chatbot />)` doesn't throw; the flows under test don't depend on
// real speech or scroll behavior.
beforeEach(() => {
  vi.clearAllMocks();
  Object.defineProperty(window, "speechSynthesis", {
    value: {
      getVoices: () => [] as SpeechSynthesisVoice[],
      cancel: () => {},
      speak: () => {},
      onvoiceschanged: undefined,
    },
    configurable: true,
  });
  Element.prototype.scrollIntoView = () => {};
});

afterEach(() => {
  vi.unstubAllGlobals();
  Element.prototype.scrollIntoView = originalScrollIntoView;
});

async function openPanel() {
  const user = userEvent.setup();
  render(<Chatbot />);
  await user.click(screen.getByRole("button", { name: "Open assistant" }));
  return user;
}

interface ChatCall {
  args: [string, { method?: string; body?: Record<string, unknown>; [k: string]: unknown }];
}

function chatPosts(): ChatCall[] {
  return fetchMock.mock.calls
    .filter(([p]) => p === "/chatbot/api")
    .map((args) => ({ args: args as ChatCall["args"] }));
}

describe("Chatbot AI-UI", () => {
  it("sends share_chart:false by default and true after toggling the opt-in", async () => {
    fetchMock.mockImplementation(async (path: string) => {
      if (path === "/chatbot/api") return jobResponse({ response: "hi" });
      return jobResponse({});
    });
    const user = await openPanel();
    const input = screen.getByRole("textbox", { name: "Assistant message" });

    await user.type(input, "hello");
    await user.click(screen.getByRole("button", { name: "Send message" }));
    await screen.findByText("hi");
    expect(chatPosts()[0].args[1].body).toEqual({ message: "hello", share_chart: false });

    await user.click(screen.getByRole("checkbox", { name: /share my chart data/i }));
    await user.type(input, "again");
    await user.click(screen.getByRole("button", { name: "Send message" }));
    expect(await screen.findAllByText("hi")).toHaveLength(2);
    expect(chatPosts()[1].args[1].body).toEqual({ message: "again", share_chart: true });
  });

  it("renders the non-clinical disclaimer under the input area", async () => {
    fetchMock.mockResolvedValue(jobResponse({}));
    await openPanel();
    expect(
      screen.getByText("General information only — not medical advice, diagnosis or treatment.")
    ).toBeInTheDocument();
  });

  it("renders the emergency card when the server flags emergency (no local keyword match)", async () => {
    // "Please help, things are bad" matches no local emergency keyword, so the
    // card can only come from the server's emergency flag.
    fetchMock.mockImplementation(async (path: string) => {
      if (path === "/chatbot/api")
        return jobResponse({ response: "Emergency reply text", emergency: true });
      return jobResponse({});
    });
    const user = await openPanel();

    await user.type(
      screen.getByRole("textbox", { name: "Assistant message" }),
      "Please help, things are bad"
    );
    await user.click(screen.getByRole("button", { name: "Send message" }));

    await screen.findByText("If this is a medical emergency, call for help now.");
    expect(screen.getByText(/Emergency numbers: 112/i)).toBeInTheDocument();
    expect(screen.getByText("Emergency reply text")).toBeInTheDocument();
  });

  it("keeps the legacy 200 {response} path working without touching the jobs API", async () => {
    fetchMock.mockImplementation(async (path: string) => {
      if (path === "/chatbot/api") return jobResponse({ response: "Legacy answer" });
      return jobResponse({});
    });
    const user = await openPanel();

    await user.type(screen.getByRole("textbox", { name: "Assistant message" }), "old path");
    await user.click(screen.getByRole("button", { name: "Send message" }));

    await screen.findByText("Legacy answer");
    expect(pollJob).not.toHaveBeenCalled();
    expect(fetchMock.mock.calls.every(([p]) => !String(p).startsWith("/api/v1/jobs/"))).toBe(true);
  });

  it("polls a 202 {job_id} and renders job.result.response as the assistant bubble", async () => {
    fetchMock.mockImplementation(async (path: string) => {
      if (path === "/chatbot/api") return jobResponse({ success: true, job_id: 42 });
      return jobResponse({});
    });
    pollMock.mockResolvedValue({
      id: "42",
      type: "ai:chat",
      status: "completed",
      result: { response: "Queued answer" },
    });
    const user = await openPanel();

    await user.type(screen.getByRole("textbox", { name: "Assistant message" }), "queued path");
    await user.click(screen.getByRole("button", { name: "Send message" }));

    expect(await screen.findByText("Queued answer")).toBeInTheDocument();
    expect(pollMock).toHaveBeenCalledTimes(1);
    expect(pollMock.mock.calls[0][0]).toBe(42);
    expect(pollMock.mock.calls[0][1]).toEqual(
      expect.objectContaining({ signal: expect.any(AbortSignal) })
    );
  });

  it("shows a retry affordance when the queued job fails, and retry re-sends the message", async () => {
    fetchMock.mockImplementation(async (path: string) => {
      if (path === "/chatbot/api") return jobResponse({ success: true, job_id: 7 });
      return jobResponse({});
    });
    pollMock.mockResolvedValue({ id: "7", type: "ai:chat", status: "failed" });
    const user = await openPanel();

    await user.type(screen.getByRole("textbox", { name: "Assistant message" }), "flaky question");
    await user.click(screen.getByRole("button", { name: "Send message" }));

    expect(await screen.findByText("The assistant could not finish that answer.")).toBeInTheDocument();
    const retry = screen.getByRole("button", { name: "Try again" });

    // Retry re-POSTs the same message; this time the job completes.
    pollMock.mockClear().mockResolvedValue({
      id: "8",
      type: "ai:chat",
      status: "completed",
      result: { response: "Retried answer" },
    });
    await user.click(retry);

    expect(await screen.findByText("Retried answer")).toBeInTheDocument();
    expect(chatPosts()[0].args[1].body).toEqual({ message: "flaky question", share_chart: false });
    expect(chatPosts()[1].args[1].body).toEqual({ message: "flaky question", share_chart: false });
  });
});
