import type { Metadata } from "next";

// Doctor console: authenticated-only area, excluded from search engines.
export const metadata: Metadata = {
  title: "Doctor console",
  robots: { index: false, follow: true },
};

export default function DoctorLayoutMeta({ children }: { children: React.ReactNode }) {
  return children;
}
