import type { Metadata, Viewport } from "next";
// import { Inter } from "next/font/google";
import "./globals.css";
import Chatbot from "@/components/Chatbot";
import Providers from "@/components/Providers";

// const inter = Inter({ subsets: ["latin"] });
const inter = { className: "font-sans" };

export const metadata: Metadata = {
  metadataBase: new URL("https://healthtrack.store"),
  title: {
    default: "HealthTrack+ — Tactile Health Management for Patients, Doctors & Providers",
    template: "%s | HealthTrack+",
  },
  description:
    "HealthTrack+ is a premium health-management platform with tactile, medical-grade tracking: vitals, smart medication regimens, neuro insights, and encrypted data vaults for patients, doctors, and providers.",
  openGraph: {
    type: "website",
    siteName: "HealthTrack+",
    title: "HealthTrack+ — Tactile Health Management for Patients, Doctors & Providers",
    description:
      "Experience the feel of premium healthcare. Tactile tracking for your body and mind — vitals, medications, and neuro insights in one secure platform.",
    url: "https://healthtrack.store/",
    locale: "en_US",
    images: [
      {
        url: "/og-image.png",
        width: 1200,
        height: 630,
        alt: "HealthTrack+ — Design Your Health Future",
      },
    ],
  },
  twitter: {
    card: "summary_large_image",
    title: "HealthTrack+ — Tactile Health Management",
    description:
      "Experience the feel of premium healthcare. Tactile tracking for your body and mind — vitals, medications, and neuro insights in one secure platform.",
    images: ["/og-image.png"],
  },
  icons: {
    icon: [
      { url: "/favicon-16x16.png", sizes: "16x16", type: "image/png" },
      { url: "/favicon-32x32.png", sizes: "32x32", type: "image/png" },
      { url: "/logo.svg", type: "image/svg+xml" },
    ],
    apple: [{ url: "/apple-touch-icon.png", sizes: "180x180", type: "image/png" }],
  },
  manifest: "/manifest.webmanifest",
  robots: {
    index: true,
    follow: true,
    googleBot: {
      index: true,
      follow: true,
    },
  },
  alternates: {
    canonical: "/",
  },
};

export const viewport: Viewport = {
  width: "device-width",
  initialScale: 1,
  themeColor: "#00bfa5",
};

export default function RootLayout({
  children,
}: Readonly<{
  children: React.ReactNode;
}>) {
  return (
    <html lang="en">
      <body className={inter.className}>
        <Providers>
          {children}
          <Chatbot />
        </Providers>
      </body>
    </html>
  );
}
