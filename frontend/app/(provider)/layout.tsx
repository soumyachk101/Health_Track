import type { Metadata } from "next";

// Provider console: authenticated-only area, excluded from search engines.
export const metadata: Metadata = {
  title: "Provider console",
  robots: { index: false, follow: true },
};

export default function ProviderLayoutMeta({ children }: { children: React.ReactNode }) {
  return children;
}
