import type { Metadata } from "next";

export const metadata: Metadata = {
  title: "Privacy Policy",
  description:
    "How HealthTrack+ collects, uses, stores, and shares your data — including your health records and AI chatbot interactions. Template pending legal review.",
  robots: {
    index: true,
    follow: true,
  },
};

export default function PrivacyPage() {
  return (
    <div className="min-h-screen bg-[#FDFDFF] text-slate-700 font-sans">
      <div className="mx-auto max-w-3xl px-5 py-12 sm:px-8 lg:py-16">
        {/* Header */}
        <div className="mb-8 space-y-4">
          <h1 className="text-4xl sm:text-5xl font-black text-slate-800 tracking-tight leading-tight">
            Privacy Policy
          </h1>
          <p className="text-slate-500 font-medium">
            What we collect, how we use it, and the choices you have.
          </p>
          <div className="card-skeuo p-4">
            <p className="text-sm font-semibold text-amber-800 leading-relaxed">
              This document is a template pending legal review. It must be
              reviewed by a qualified legal professional before publication.
            </p>
          </div>
          <p className="text-xs font-bold uppercase tracking-widest text-slate-400">
            Last updated: 【date】
          </p>
        </div>

        {/* Intro */}
        <p className="mb-8 text-base leading-relaxed text-slate-600">
          This Privacy Policy explains how HealthTrack+ collects, uses, stores,
          and shares information when you use our app and services. It describes
          the practices that are actually in place today, and flags the items
          that still need to be completed or legally confirmed. We only describe
          data practices that are real for this service; where a value is
          business- or legal-specific, it is marked as a placeholder rather than
          guessed.
        </p>

        <div className="space-y-6">
          {/* 1. Data we collect */}
          <Section number="1" title="What data we collect and why">
            <List
              items={[
                "Account details: your username, email address, and password (used to verify your identity and run your account).",
                "Health records you enter: blood pressure, blood sugar, weight, heart rate, temperature, oxygen, and notes.",
                "Medication and care information: medicines, prescriptions, insurance policies, past records, appointments, and service requests.",
                "Mental-health logs and lifestyle logs you choose to record.",
                "Activity logs describing how you use the app.",
                "AI chatbot messages: when you use the health chatbot, the text you send is passed to a third-party AI provider to generate a response.",
              ]}
            />
            <p className="mt-3 text-sm text-slate-500 leading-relaxed">
              We collect this data so we can deliver the features you use and
              scope your information to your own account.
            </p>
          </Section>

          {/* 2. How we use it */}
          <Section number="2" title="How we use your data">
            <List
              items={[
                "To create, secure, and maintain your account and authenticate you on each session.",
                "To store the health, medication, and appointment information you record and to show it back to you.",
                "To power the AI health chatbot and optional text-to-speech / voice input features.",
                "To send transactional email, such as verification and password-reset codes.",
                "To support administrators in operating and maintaining the service.",
              ]}
            />
          </Section>

          {/* 3. Where it is stored */}
          <Section number="3" title="Where your data is stored">
            <p className="text-base text-slate-600 leading-relaxed">
              Account and health data is stored server-side in a PostgreSQL
              database. When you log in, a signed session token (JWT) is issued
              and stored on your device in browser local storage.
            </p>
            <p className="mt-3 text-sm text-slate-500 leading-relaxed">
              The hosting location / region and data-residency details for this
              database are a business-specific value: 【Owner to supply:
              hosting provider, data location / region, and data-residency
              details】.
            </p>
          </Section>

          {/* 4. Sharing */}
          <Section number="4" title="When your data is shared with others">
            <List
              items={[
                "AI providers: your chatbot messages are sent to third-party AI services (Groq is the primary provider, with OpenRouter / OpenAI as fallbacks) to generate responses. An optional voice provider may be used for text-to-speech.",
                "Email provider: your email address is used to send verification and password-reset emails via SMTP.",
                "Google: if you sign in with Google, Google handles the OAuth sign-in flow. Google does not receive your health data.",
              ]}
            />
            <p className="mt-3 text-sm text-slate-500 leading-relaxed">
              In-browser speech recognition and web-speech synthesis run on your
              device. We do not sell your personal or health data.
            </p>
            <p className="mt-3 text-sm font-semibold text-amber-800">
              Legal review needed: the specific data each AI and voice provider
              receives, and the terms under which they process it, must be
              confirmed and documented before publication.
            </p>
          </Section>

          {/* 5. Roles & scoping */}
          <Section number="5" title="Roles and access to your data">
            <p className="text-base text-slate-600 leading-relaxed">
              The service supports the roles patient, doctor, provider, and
              admin. Data is scoped to the logged-in user. Administrators have
              elevated access for the purposes of operating the service.
            </p>
          </Section>

          {/* 6. Your rights */}
          <Section number="6" title="Your rights and choices">
            <List
              items={[
                "Access: view the data stored on your account.",
                "Correct: update or fix your information where the interface allows it.",
                "Delete: request deletion of your account and associated data.",
                "Export: request a copy of your data.",
                "Revoke consent: withdraw consent for processing and sign out / close your account.",
              ]}
            />
            <p className="mt-3 text-sm font-semibold text-amber-800">
              How each of these requests is fulfilled (timelines, supported
              formats, whether export and deletion are fully available today)
              is a business-specific detail that must be confirmed and is a
              placeholder: 【Owner to supply: how users exercise these rights】.
            </p>
          </Section>

          {/* 7. Retention */}
          <Section number="7" title="Retention">
            <p className="text-base text-slate-600 leading-relaxed">
              How long we retain your data, and when it is deleted, is a
              business/legal decision that has not been set here.
            </p>
            <p className="mt-3 text-sm font-semibold text-amber-800">
              【Owner to supply: data retention period and deletion policy】
            </p>
          </Section>

          {/* 8. Security */}
          <Section number="8" title="Security">
            <p className="text-base text-slate-600 leading-relaxed">
              Data is protected in transit and at rest using standard practices
              for authentication and storage. No single control makes a system
              completely secure; if you believe your account has been
              compromised, contact us using the details below.
            </p>
            <p className="mt-3 text-sm font-semibold text-amber-800">
              Any specific security controls, certifications, or audit
              statements must not be claimed unless they are genuinely in place
              and can be verified: 【To be confirmed】.
            </p>
          </Section>

          {/* 9. Children */}
          <Section number="9" title="Children">
            <p className="text-base text-slate-600 leading-relaxed">
              The service is not directed at children, and we do not knowingly
              collect data from them. If you believe a child has provided data,
              contact us so it can be addressed.
            </p>
            <p className="mt-3 text-sm font-semibold text-amber-800">
              The applicable age threshold and any related obligations are
              legal-specific: 【Owner to supply: minimum age and compliance
              approach】.
            </p>
          </Section>

          {/* 10. Changes */}
          <Section number="10" title="Changes to this policy">
            <p className="text-base text-slate-600 leading-relaxed">
              We may update this policy as the service changes. When we do, we
              will update the “Last updated” date at the top of this page.
            </p>
          </Section>

          {/* 11. Contact */}
          <Section number="11" title="Contact">
            <p className="text-base text-slate-600 leading-relaxed">
              Questions, rights requests, or data-protection concerns can be
              sent to the contact below.
            </p>
            <p className="mt-3 text-sm font-semibold text-amber-800">
              【Owner to supply: contact email / data-protection officer】
            </p>
          </Section>

          {/* Ad / analytics clarification */}
          <Section number="" title="Advertising and analytics">
            <p className="text-base text-slate-600 leading-relaxed">
              We do not use third-party advertising cookies. There is no web
              analytics service currently installed. This may change; if it
              does, the policy will be updated.
            </p>
          </Section>
        </div>

        <p className="mt-10 text-xs font-semibold text-amber-800 leading-relaxed">
          This document is a template pending legal review. It must be reviewed
          by a qualified legal professional before publication. All 【Owner to
          supply】 and 【To be confirmed】 placeholders must be completed, and any
          claim about a certification, legal compliance, or security control
          must be verified as true for this service.
        </p>
      </div>
    </div>
  );
}

/* --- Small presentational helpers (server-safe, pure JSX) --- */

function Section({
  number,
  title,
  children,
}: {
  number?: string;
  title: string;
  children: React.ReactNode;
}) {
  return (
    <section className="card-skeuo space-y-3">
      <h2 className="text-xl font-black text-slate-800 tracking-tight">
        {number ? `${number}. ` : ""}
        {title}
      </h2>
      {children}
    </section>
  );
}

function List({ items }: { items: string[] }) {
  return (
    <ul className="space-y-2">
      {items.map((item, i) => (
        <li key={i} className="flex gap-3 text-sm text-slate-600 leading-relaxed">
          <span className="mt-2 h-1.5 w-1.5 shrink-0 rounded-full bg-teal-500" />
          <span>{item}</span>
        </li>
      ))}
    </ul>
  );
}
