import type { Metadata } from "next";

// Admin console: privileged area, excluded from search engines.
export const metadata: Metadata = {
  title: "Admin console",
  robots: { index: false, follow: true },
};

export default function AdminLayoutMeta({ children }: { children: React.ReactNode }) {
  return children;
}
