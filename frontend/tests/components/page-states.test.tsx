import { describe, expect, it, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { PageLoading, PageError, PageEmpty, PageSkeleton, ErrorBanner, SuccessBanner, FieldError } from "@/components/ui/page-states";

// Keep next/link working for the real (PageEmpty renders a Link).
vi.mock("next/link", () => ({
  default: ({ children, ...props }: React.PropsWithChildren<React.AnchorHTMLAttributes<HTMLAnchorElement>>) => (
    <a {...props}>{children}</a>
  ),
}));

describe("page-states", () => {
  it("PageLoading announces itself to screen readers and shows the label", () => {
    render(<PageLoading label="Loading dashboard" />);
    const status = screen.getByRole("status");
    expect(status).toHaveTextContent("Loading dashboard");
    // The spinner is decorative.
    expect(status.querySelector(".animate-spin")).toHaveAttribute("aria-hidden", "true");
  });

  it("PageError shows the message and a retry button that calls onRetry", async () => {
    const user = userEvent.setup();
    const onRetry = vi.fn();
    render(<PageError message="Failed to load" onRetry={onRetry} />);
    expect(screen.getByRole("alert")).toHaveTextContent("Failed to load");
    const btn = screen.getByRole("button", { name: /retry/i });
    expect(btn).toHaveAttribute("aria-label", "Retry: Failed to load");
    await user.click(btn);
    expect(onRetry).toHaveBeenCalledTimes(1);
  });

  it("PageEmpty renders title/description and a CTA link when provided", () => {
    render(<PageEmpty title="No medicines yet" description="Add your first medicine." ctaLabel="Add medicine" ctaHref="/medicines/add" />);
    expect(screen.getByText("No medicines yet")).toBeTruthy();
    expect(screen.getByText("Add your first medicine.")).toBeTruthy();
    expect(screen.getByRole("link", { name: /add medicine/i })).toHaveAttribute("href", "/medicines/add");
  });

  it("PageEmpty omits the CTA when either label or href is missing", () => {
    render(<PageEmpty title="Empty" />);
    expect(screen.queryByRole("link")).toBeNull();
  });

  it("PageSkeleton renders the right number of stat and row blocks", () => {
    const { container } = render(<PageSkeleton stats={4} rows={2} title="Loading dashboard" />);
    // The title is exposed via aria-label, not visible text.
    expect(container.querySelector("[aria-label]")).toHaveAttribute("aria-label", "Loading dashboard");
    // 1 header block + 4 stat cards + 2 rows -> 7 pulsing blocks total.
    expect(container.querySelectorAll(".animate-pulse").length).toBe(7);
  });

  it("Error/Success banners carry the correct ARIA roles", () => {
    const { rerender } = render(<ErrorBanner>Something failed</ErrorBanner>);
    expect(screen.getByRole("alert")).toHaveTextContent("Something failed");
    rerender(<SuccessBanner>Done</SuccessBanner>);
    expect(screen.getByRole("status")).toHaveTextContent("Done");
  });

  it("FieldError renders only when a message is provided and keeps the id for aria-describedby", () => {
    const { container } = render(<FieldError id="name-err" />);
    expect(container).toBeEmptyDOMElement();
    const { unmount } = render(<FieldError id="name-err" message="Required" />);
    const p = screen.getByRole("alert");
    expect(p).toHaveAttribute("id", "name-err");
    expect(p).toHaveTextContent("Required");
    unmount();
  });
});
