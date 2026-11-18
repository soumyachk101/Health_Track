import type { Metadata } from "next";
import { RouteGuard } from "@/components/guards/RouteGuard";

// Patient dashboard: authenticated-only content, never indexed.
export const metadata: Metadata = {
  title: "Patient dashboard",
  robots: { index: false, follow: true },
};

export default function DashboardLayoutMeta({ children }: { children: React.ReactNode }) {
  return (
    <RouteGuard requiredRole="patient">
      {children}
    </RouteGuard>
  );
}
