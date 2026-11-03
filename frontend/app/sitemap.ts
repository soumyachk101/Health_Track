import type { MetadataRoute } from "next";

// Only indexable, public pages are listed. Auth and role-gated pages
// (login, register, dashboard, admin, doctor, provider) are deliberately
// excluded and are noindexed per-page.
export default function sitemap(): MetadataRoute.Sitemap {
  const domain = "https://healthtrack.store";
  const now = new Date();

  return [
    {
      url: `${domain}/`,
      lastModified: now,
      changeFrequency: "weekly",
      priority: 1.0,
    },
    {
      url: `${domain}/privacy`,
      lastModified: now,
      changeFrequency: "yearly",
      priority: 0.3,
    },
    {
      url: `${domain}/terms`,
      lastModified: now,
      changeFrequency: "yearly",
      priority: 0.3,
    },
  ];
}
