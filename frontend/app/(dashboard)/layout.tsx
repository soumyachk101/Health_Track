import type { Metadata } from "next";

// Patient dashboard: authenticated-only content, never indexed.
export const metadata: Metadata = {
  title: "Patient dashboard",
  robots: { index: false, follow: true },
};

export default function DashboardLayoutMeta({ children }: { children: React.ReactNode }) {
  return children;
}
