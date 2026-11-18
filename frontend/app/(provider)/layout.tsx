import type { Metadata } from "next";
import { RouteGuard } from "@/components/guards/RouteGuard";

// Provider console: authenticated-only area, excluded from search engines.
export const metadata: Metadata = {
  title: "Provider console",
  robots: { index: false, follow: true },
};

export default function ProviderLayoutMeta({ children }: { children: React.ReactNode }) {
  return (
    <RouteGuard requiredRole="provider">
      {children}
    </RouteGuard>
  );
}
