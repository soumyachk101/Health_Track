import type { Metadata } from "next";

export const metadata: Metadata = {
  title: "Terms of Service",
  description:
    "Terms for using HealthTrack+: eligible use, accounts, your content and IP, disclaimers for AI output, termination, liability, and governing law. Template pending legal review.",
  robots: {
    index: true,
    follow: true,
  },
};

export default function TermsPage() {
  return (
    <div className="min-h-screen bg-[#FDFDFF] text-slate-700 font-sans">
      <div className="mx-auto max-w-3xl px-5 py-12 sm:px-8 lg:py-16">
        {/* Header */}
        <div className="mb-8 space-y-4">
          <h1 className="text-4xl sm:text-5xl font-black text-slate-800 tracking-tight leading-tight">
            Terms of Service
          </h1>
          <p className="text-slate-500 font-medium">
            The rules that govern your use of HealthTrack+.
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
          These terms describe what you can and cannot do with the HealthTrack+
          service. They are written to be honest about the features that
          actually exist today, and they flag the items that still need a
          business or legal decision. By using the service you agree to these
          terms; if you do not agree, do not use it.
        </p>

        <div className="space-y-6">
          {/* 1. Eligible use */}
          <Section number="1" title="Eligible use">
            <p className="text-base text-slate-600 leading-relaxed">
              The service is for individuals and professionals managing their
              own health or care information, and for the roles the service
              supports (patient, doctor, provider, and admin). You must be old
              enough to use it and to give consent, and you must provide
              accurate information.
            </p>
            <p className="mt-3 text-sm font-semibold text-amber-800">
              Any minimum-age requirement is a legal-specific value:
              【Owner to supply: minimum age / consent requirements】.
            </p>
          </Section>

          {/* 2. Account & responsibility */}
          <Section number="2" title="Account and responsibility">
            <List
              items={[
                "You are responsible for keeping your password private and for all activity that happens under your account.",
                "You must keep your account details accurate and up to date.",
                "We issue a signed session token on login; treat it as confidential, do not share it, and sign out when you are done.",
                "You may only use one personal account unless the service explicitly supports additional roles for you.",
              ]}
            />
            <p className="mt-3 text-sm text-slate-500 leading-relaxed">
              Notify us promptly if your account is compromised. You are
              responsible for your own actions within the service, including
              how you use and interpret its output.
            </p>
          </Section>

          {/* 3. User content & IP */}
          <Section number="3" title="User content, data ownership, and IP">
            <List
              items={[
                "Health data you enter (vitals, medicines, prescriptions, mental-health and lifestyle logs, appointments, service requests) remains yours. We do not sell your health data.",
                "By submitting content, you grant us a license to store and process it so the service can work for you.",
                "The service, its interface, and its original elements are owned by us, and you may not remove or alter them, or use them to build a competing service.",
                "You keep ownership of your own content, subject to the license above.",
              ]}
            />
            <p className="mt-3 text-sm text-slate-500 leading-relaxed">
              The precise scope of the license, and whether it survives
              deletion or export, is a legal decision: 【Owner to supply:
              scope of the content license】.
            </p>
          </Section>

          {/* 4. Availability & disclaimers */}
          <Section number="4" title="Service availability and disclaimers">
            <p className="text-base text-slate-600 leading-relaxed">
              The service is provided on an “as is” basis. We aim for
              reliability, but we do not guarantee it will be uninterrupted,
              error-free, or available at all times. Parts of the service can
              be paused or changed.
            </p>
          </Section>

          {/* 5. AI disclaimer (prominent) */}
          <div className="card-skeuo border-2 border-teal-200 space-y-3">
            <div className="flex items-center gap-3">
              <div className="h-10 w-10 rounded-2xl bg-teal-50 flex items-center justify-center text-teal-600 shrink-0">
                <svg
                  className="h-5 w-5"
                  fill="none"
                  viewBox="0 0 24 24"
                  stroke="currentColor"
                  strokeWidth="2.5"
                  aria-hidden="true"
                >
                  <path
                    strokeLinecap="round"
                    strokeLinejoin="round"
                    d="M12 9v3.75m-9.303 3.376c-.866 1.5.217 3.374 1.948 3.374h14.71c1.73 0 2.813-1.874 1.948-3.374L3.378 3.124a2.25 2.25 0 00-1.948-1.234c-.836 0-1.448.54-1.63 1.318L.668 18.46A2.25 2.25 0 002.25 20.71h.002l.002-.002h.002c.836 0 1.448-.54 1.63-1.318l.002-.002zm14.62 0c.866 1.5-.217 3.374-1.948 3.374H3.378c-1.73 0-2.813-1.874-1.948-3.374L12 3.124l9.022 14.632z"
                  />
                </svg>
              </div>
              <h2 className="text-lg font-black text-slate-800 tracking-tight">
                Important: this is not medical advice
              </h2>
            </div>
            <p className="text-base text-slate-700 font-medium leading-relaxed">
              The AI health chatbot provides <strong>general information only</strong>. It does
              not provide medical advice, diagnosis, or treatment, and it is
              not a substitute for professional medical care. Always consult a
              qualified healthcare professional about any medical concern, and
              do not delay or disregard their advice because of something you
              saw here.
            </p>
            <p className="text-sm text-slate-500 leading-relaxed">
              In an emergency, contact your local emergency services
              immediately.
            </p>
          </div>

          {/* 5. No liability for reliance on AI */}
          <Section number="5" title="No liability for reliance on AI output">
            <p className="text-base text-slate-600 leading-relaxed">
              Because the AI chatbot can be inaccurate, you rely on its output
              at your own risk. We are not responsible for decisions you make
              based on information generated by the chatbot or any other part
              of the service.
            </p>
          </Section>

          {/* 7. Termination */}
          <Section number="6" title="Account termination">
            <List
              items={[
                "You may close your own account at any time.",
                "We may suspend or terminate an account or access if you breach these terms or for legal / safety reasons.",
                "On termination, your right to use the service stops, and how long your data is kept is set by the retention policy: 【Owner to supply: retention / post-termination handling】.",
              ]}
            />
          </Section>

          {/* 8. Limitation of liability */}
          <Section number="7" title="Limitation of liability">
            <p className="text-base text-slate-600 leading-relaxed">
              To the maximum extent permitted by law, we are not liable for
              indirect, incidental, or consequential damages, or for loss of
              data, profits, or business, arising from your use of the service.
              The overall liability we accept is limited to the maximum amount
              the law allows us to cap it at.
            </p>
            <p className="mt-3 text-sm font-semibold text-amber-800">
              Whether this limitation is appropriate, and the exact cap, is a
              legal decision that must be confirmed: 【Owner to supply:
              liability cap and any carve-outs】.
            </p>
          </Section>

          {/* 9. Governing law */}
          <Section number="8" title="Governing law and jurisdiction">
            <p className="text-base text-slate-600 leading-relaxed">
              These terms are governed by the law of, and disputes are heard
              in the courts of, the jurisdiction set below.
            </p>
            <p className="mt-3 text-sm font-semibold text-amber-800">
              【Owner to supply: governing law and jurisdiction】
            </p>
          </Section>

          {/* 10. Changes to terms */}
          <Section number="9" title="Changes to these terms">
            <p className="text-base text-slate-600 leading-relaxed">
              We may update these terms as the service changes. When we do, we
              will update the “Last updated” date at the top of this page.
              Continued use after a change means you accept the new terms.
            </p>
          </Section>

          {/* 11. Contact */}
          <Section number="10" title="Contact">
            <p className="text-base text-slate-600 leading-relaxed">
              Questions about these terms can be sent to the contact below.
            </p>
            <p className="mt-3 text-sm font-semibold text-amber-800">
              【Owner to supply: contact email / legal contact】
            </p>
          </Section>
        </div>

        <p className="mt-10 text-xs font-semibold text-amber-800 leading-relaxed">
          This document is a template pending legal review. It must be reviewed
          by a qualified legal professional before publication. All 【Owner to
          supply】 placeholders must be completed, and the AI / medical
          disclaimers should be checked by a professional in the jurisdictions
          where the service is offered.
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
