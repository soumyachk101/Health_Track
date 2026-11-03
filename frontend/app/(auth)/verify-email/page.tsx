"use client";

import { useState, useEffect, Suspense, useRef } from "react"
import { Sparkles, CheckCircle2, ArrowRight, AlertCircle, RefreshCw, MailOpen } from "lucide-react"
import { useRouter } from "next/navigation"
import { supaSession, supaResendVerification, SupabaseNotConfiguredError } from "@/lib/supabase"
import { motion, AnimatePresence } from "framer-motion"
import { Button } from "@/components/ui/button"
import { cn } from "@/lib/utils"

// Supabase handles email confirmation with its own link (which re-authenticates
// the browser via detectSessionInUrl). When our site later loads at this route
// with a live Supabase session, the email is verified. Otherwise the user must
// open the link from their inbox — with a resend as a fallback.
function VerifyEmailContent() {
    const router = useRouter()
    const [status, setStatus] = useState<'verifying' | 'success' | 'error'>('verifying')
    const [error, setError] = useState<string | null>(null)
    const [resendStatus, setResendStatus] = useState<'idle' | 'loading' | 'success'>('idle')
    const [resendEmail, setResendEmail] = useState("")
    const statusRegionRef = useRef<HTMLDivElement>(null)

    useEffect(() => {
        let cancelled = false
        supaSession()
            .then((info) => {
                if (cancelled) return
                if (info) {
                    setStatus('success')
                } else {
                    setStatus('error')
                    setError("Open the confirmation link from your email to verify your address.")
                }
            })
            .catch(() => {
                if (cancelled) return
                setStatus('error')
                setError("We couldn't check your verification status right now. Please try again.")
            })
        return () => { cancelled = true }
    }, [])

    // Announce status changes to screen readers and move focus there.
    useEffect(() => {
        if (status !== 'verifying') statusRegionRef.current?.focus()
    }, [status])

    const handleResend = async (email?: string) => {
        const target = email ?? resendEmail
        if (!target) {
            setError("Enter your email address so we can send a fresh verification link.")
            return
        }
        setResendStatus('loading')
        try {
            await supaResendVerification(target)
            setResendStatus('success')
            setTimeout(() => setResendStatus('idle'), 5000)
        } catch (err: unknown) {
            setError(err instanceof SupabaseNotConfiguredError
                ? "Verification resend is unavailable right now. Please try again shortly."
                : "Could not send the verification email. Please try again.")
            setResendStatus('idle')
        }
    }

    return (
        <div className="min-h-screen flex items-center justify-center bg-[#FDFDFF] text-slate-700 font-sans relative overflow-hidden">
            {/* Background Aesthetics */}
            <div className="absolute inset-0 bg-[radial-gradient(circle_at_50%_50%,#f0f9ff_0%,transparent_100%)] opacity-50"></div>
            <div className="absolute top-0 left-0 w-full h-1 bg-gradient-to-r from-teal-500 via-blue-500 to-indigo-500"></div>

            <div className="w-full max-w-lg px-8 relative z-10">
                <motion.div
                    initial={{ opacity: 0, y: 20 }}
                    animate={{ opacity: 1, y: 0 }}
                    className="bg-white rounded-[3rem] shadow-skeuo-floating border border-slate-50 p-12 text-center space-y-10"
                >
                    {/* Header Icon */}
                    <div className="relative mx-auto w-24 h-24">
                        <AnimatePresence mode="wait">
                            {status === 'verifying' && (
                                <motion.div
                                    key="verifying"
                                    initial={{ opacity: 0, scale: 0.5 }}
                                    animate={{ opacity: 1, scale: 1 }}
                                    exit={{ opacity: 0, scale: 0.5 }}
                                    className="absolute inset-0 flex items-center justify-center"
                                >
                                    <div className="absolute inset-0 bg-teal-500/10 rounded-3xl animate-pulse"></div>
                                    <RefreshCw className="h-12 w-12 text-teal-600 animate-spin" />
                                </motion.div>
                            )}
                            {status === 'success' && (
                                <motion.div
                                    key="success"
                                    initial={{ opacity: 0, scale: 0.5 }}
                                    animate={{ opacity: 1, scale: 1 }}
                                    className="absolute inset-0 flex items-center justify-center"
                                >
                                    <div className="absolute inset-0 bg-emerald-500/10 rounded-3xl animate-bounce"></div>
                                    <CheckCircle2 className="h-14 w-14 text-emerald-500" />
                                </motion.div>
                            )}
                            {status === 'error' && (
                                <motion.div
                                    key="error"
                                    initial={{ opacity: 0, scale: 0.5 }}
                                    animate={{ opacity: 1, scale: 1 }}
                                    className="absolute inset-0 flex items-center justify-center"
                                >
                                    <div className="absolute inset-0 bg-amber-500/10 rounded-3xl"></div>
                                    <MailOpen className="h-12 w-12 text-amber-500" />
                                </motion.div>
                            )}
                        </AnimatePresence>
                    </div>

                    <div className="space-y-4">
                        <div className="flex items-center justify-center gap-2">
                            <Sparkles className="h-5 w-5 text-teal-500" />
                            <span className="text-xl font-black text-slate-800 tracking-tighter uppercase">HealthTrack+ Elite</span>
                        </div>

                        <div
                            ref={statusRegionRef}
                            tabIndex={-1}
                            role={status === "error" ? "alert" : "status"}
                            aria-busy={status === "verifying" || undefined}
                            className="outline-none"
                        >
                            <AnimatePresence mode="wait">
                                {status === 'verifying' && (
                                    <motion.div key="v-text" initial={{ opacity: 0 }} animate={{ opacity: 1 }} exit={{ opacity: 0 }}>
                                        <h2 className="text-4xl font-black text-slate-800 tracking-tight">Checking Verification</h2>
                                        <p className="text-slate-400 font-bold text-lg mt-2">Confirming your email address...</p>
                                    </motion.div>
                                )}
                                {status === 'success' && (
                                    <motion.div key="s-text" initial={{ opacity: 0 }} animate={{ opacity: 1 }}>
                                        <h2 className="text-4xl font-black text-slate-800 tracking-tight">Email Verified</h2>
                                        <p className="text-slate-400 font-bold text-lg mt-2">Your account is active — you can sign in now.</p>
                                    </motion.div>
                                )}
                                {status === 'error' && (
                                    <motion.div key="e-text" initial={{ opacity: 0 }} animate={{ opacity: 1 }}>
                                        <h2 className="text-4xl font-black text-slate-800 tracking-tight">One Step Away</h2>
                                        <p className="text-amber-600 font-bold text-lg mt-2">{error}</p>
                                    </motion.div>
                                )}
                            </AnimatePresence>
                        </div>
                    </div>

                    <div className="pt-6 space-y-4">
                        {status === 'success' ? (
                            <Button
                                onClick={() => router.push('/login')}
                                className={cn(
                                    "w-full h-16 text-white font-black text-lg rounded-2xl transition-all shadow-xl group",
                                    "bg-teal-600 shadow-teal-500/20"
                                )}
                            >
                                <span className="flex items-center justify-center gap-3">
                                    Please Log In <ArrowRight className="h-5 w-5 group-hover:translate-x-1 transition-transform" />
                                </span>
                            </Button>
                        ) : (
                            <div className="space-y-4">
                                {status === 'error' && (
                                    <div className="flex items-center gap-2">
                                        <input
                                            id="resend-email"
                                            type="email"
                                            aria-label="Email to send the verification link to"
                                            placeholder="you@example.com"
                                            value={resendEmail}
                                            onChange={(e) => setResendEmail(e.target.value)}
                                            className="flex-1 h-14 rounded-2xl border border-slate-200 bg-slate-50/50 px-5 text-base font-medium outline-none focus:ring-[4px] focus:ring-slate-100"
                                        />
                                        <button
                                            type="button"
                                            onClick={() => handleResend()}
                                            disabled={resendStatus === 'loading'}
                                            className={cn(
                                                "flex items-center gap-2 px-5 h-14 rounded-2xl font-black text-xs uppercase tracking-widest transition-all",
                                                "bg-slate-800 text-white hover:opacity-90 disabled:opacity-50"
                                            )}
                                        >
                                            {resendStatus === 'loading'
                                                ? <RefreshCw className="h-4 w-4 animate-spin" />
                                                : <AlertCircle className="h-4 w-4" />}
                                            {resendStatus === 'success' ? "Sent!" : "Resend"}
                                        </button>
                                    </div>
                                )}
                                {resendStatus === 'success' && (
                                    <p role="status" className="text-center text-sm font-bold text-emerald-600">
                                        Verification link sent to {resendEmail}.
                                    </p>
                                )}
                                <Button
                                    onClick={() => router.push('/login')}
                                    className={cn(
                                        "w-full h-16 text-white font-black text-lg rounded-2xl transition-all shadow-xl group",
                                        "bg-slate-800 shadow-slate-500/20"
                                    )}
                                >
                                    <span className="flex items-center justify-center gap-3">
                                        Return to Login <ArrowRight className="h-5 w-5 group-hover:translate-x-1 transition-transform" />
                                    </span>
                                </Button>
                            </div>
                        )}
                    </div>
                </motion.div>

                <div className="mt-12 text-center">
                    <p className="text-[10px] font-black uppercase tracking-[0.4em] text-slate-300">
                        Secure Cryptographic Verification • Node v8.4.2
                    </p>
                </div>
            </div>
        </div>
    )
}

export default function VerifyEmail() {
    return (
        <Suspense fallback={
            <div className="min-h-screen flex items-center justify-center bg-white font-black tracking-widest text-slate-200">
                INIT_VERIFICATION_PROTOCOL...
            </div>
        }>
            <VerifyEmailContent />
        </Suspense>
    )
}
