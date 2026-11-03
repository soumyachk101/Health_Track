import type { Metadata } from "next";

// Auth pages are private: keep them out of search indexes while still
// allowing crawlers to follow links to the public pages.
export const metadata: Metadata = {
  title: "Sign in and account access",
  robots: { index: false, follow: true },
};

export default function AuthLayout({ children }: { children: React.ReactNode }) {
  return children;
}
