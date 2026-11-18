"use client";

import { useState, useEffect, useRef, Suspense } from "react"
import { Sparkles, CheckCircle2, ArrowRight, Loader2, MailOpen, ShieldCheck, Mail } from "lucide-react"
import { useRouter, useSearchParams } from "next/navigation"
import { apiOtpVerify, ApiError, type AuthUser } from "@/lib/api"
import { roleHomePath } from "@/lib/utils"
import { motion, AnimatePresence } from "framer-motion"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { cn } from "@/lib/utils"

// The Pure Go backend verifies signups with an emailed one-time code
// (POST /api/v1/auth/otp/verify). Enter email + code to activate the account;
// on success the backend sets the session cookies and returns the user.
function VerifyEmailContent() {
    const router = useRouter()
    const searchParams = useSearchParams()
    const [status, setStatus] = useState<'verify' | 'success' | 'error'>('verify')
    const [error, setError] = useState<string | null>(null)
    const [isVerifying, setIsVerifying] = useState(false)
    const [email, setEmail] = useState(searchParams.get("email") ?? "")
    const [code, setCode] = useState(searchParams.get("code") ?? "")
    const statusRegionRef = useRef<HTMLDivElement>(null)

    // Announce status changes to screen readers and move focus there.
    useEffect(() => {
        if (status !== 'verify') statusRegionRef.current?.focus()
    }, [status])

    const handleVerify = async (e: React.FormEvent) => {
        e.preventDefault()
        setError(null)
        if (!email.trim() || !code.trim()) {
            setError("Enter your email address and the verification code from your inbox.")
            return
        }
        setIsVerifying(true)
        try {
            const result = await apiOtpVerify(email.trim(), code.trim(), "register")
            const user: AuthUser | undefined = result.user
            // Verified: the backend issued the session cookies; route home.
            setStatus('success')
            router.push(roleHomePath(user?.role || "patient"))
        } catch (err: unknown) {
            setStatus('error')
            setError(
                err instanceof ApiError
                    ? "That code didn't work. Check it and try again, or request a new code."
                    : "We couldn't verify that code right now. Please try again."
            )
        } finally {
            setIsVerifying(false)
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
                    <div className="relative mx-auto w-24 h-24 flex items-center justify-center">
                        {status === 'success' ? (
                            <>
                                <div className="absolute inset-0 bg-emerald-500/10 rounded-3xl animate-bounce"></div>
                                <CheckCircle2 className="h-14 w-14 text-emerald-500" />
                            </>
                        ) : status === 'error' ? (
                            <>
                                <div className="absolute inset-0 bg-amber-500/10 rounded-3xl"></div>
                                <MailOpen className="h-12 w-12 text-amber-500" />
                            </>
                        ) : (
                            <>
                                <div className="absolute inset-0 bg-teal-500/10 rounded-3xl animate-pulse"></div>
                                <ShieldCheck className="h-12 w-12 text-teal-600" />
                            </>
                        )}
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
                            aria-busy={isVerifying || undefined}
                            className="outline-none"
                        >
                            <AnimatePresence mode="wait">
                                {status === 'verify' && (
                                    <motion.div key="v-text" initial={{ opacity: 0 }} animate={{ opacity: 1 }} exit={{ opacity: 0 }}>
                                        <h2 className="text-4xl font-black text-slate-800 tracking-tight">Verify Your Email</h2>
                                        <p className="text-slate-400 font-bold text-lg mt-2">Enter the 6-digit code we sent to your inbox.</p>
                                    </motion.div>
                                )}
                                {status === 'success' && (
                                    <motion.div key="s-text" initial={{ opacity: 0 }} animate={{ opacity: 1 }} exit={{ opacity: 0 }}>
                                        <h2 className="text-4xl font-black text-slate-800 tracking-tight">Email Verified</h2>
                                        <p className="text-slate-400 font-bold text-lg mt-2">Your account is active — taking you in now.</p>
                                    </motion.div>
                                )}
                                {status === 'error' && (
                                    <motion.div key="e-text" initial={{ opacity: 0 }} animate={{ opacity: 1 }} exit={{ opacity: 0 }}>
                                        <h2 className="text-4xl font-black text-slate-800 tracking-tight">One Step Away</h2>
                                        <p className="text-amber-600 font-bold text-lg mt-2">{error}</p>
                                    </motion.div>
                                )}
                            </AnimatePresence>
                        </div>
                    </div>

                    {status !== 'success' ? (
                        <form onSubmit={handleVerify} noValidate className="space-y-8 text-left">
                            <div className="space-y-3">
                                <Label htmlFor="verify-email" className="text-xs font-black text-slate-400 uppercase tracking-[0.2em] ml-2">Email Address</Label>
                                <div className="relative">
                                    <div className={cn("absolute left-5 top-1/2 -translate-y-1/2 transition-colors duration-300", email ? "text-teal-600" : "text-slate-300")}>
                                        <Mail className="h-6 w-6" />
                                    </div>
                                    <Input
                                        id="verify-email"
                                        type="email"
                                        required
                                        aria-label="Email address being verified"
                                        placeholder="you@example.com"
                                        value={email}
                                        onChange={(e) => setEmail(e.target.value)}
                                        className="h-16 pl-14 pr-6 bg-slate-50/50 border-transparent rounded-2xl focus:bg-white focus:ring-[6px] focus:ring-slate-100 focus:border-slate-200 transition-all text-lg font-medium shadow-none"
                                        autoComplete="email"
                                    />
                                </div>
                            </div>

                            <div className="space-y-3">
                                <Label htmlFor="verify-code" className="text-xs font-black text-slate-400 uppercase tracking-[0.2em] ml-2">Verification Code</Label>
                                <div className="relative">
                                    <div className={cn("absolute left-5 top-1/2 -translate-y-1/2 transition-colors duration-300", code ? "text-teal-600" : "text-slate-300")}>
                                        <ShieldCheck className="h-6 w-6" />
                                    </div>
                                    <Input
                                        id="verify-code"
                                        type="text"
                                        required
                                        aria-label="6-digit verification code"
                                        placeholder="123456"
                                        value={code}
                                        onChange={(e) => setCode(e.target.value)}
                                        className="h-16 pl-14 pr-6 bg-slate-50/50 border-transparent rounded-2xl focus:bg-white focus:ring-[6px] focus:ring-slate-100 focus:border-slate-200 transition-all text-lg font-medium shadow-none tracking-[0.3em]"
                                        autoComplete="one-time-code"
                                        inputMode="numeric"
                                    />
                                </div>
                            </div>

                            <Button
                                type="submit"
                                aria-busy={isVerifying || undefined}
                                disabled={isVerifying}
                                className="w-full h-16 bg-teal-600 text-white font-black text-lg rounded-2xl shadow-xl shadow-teal-500/20 transition-all hover:scale-[1.01] active:scale-[0.98] disabled:opacity-70 group"
                            >
                                <span className="flex items-center justify-center gap-3">
                                    {isVerifying ? (
                                        <Loader2 className="h-5 w-5 animate-spin" />
                                    ) : (
                                        <ArrowRight className="h-5 w-5 group-hover:translate-x-1 transition-transform" />
                                    )}
                                    {isVerifying ? "Verifying…" : "Verify & Continue"}
                                </span>
                            </Button>
                        </form>
                    ) : null}

                    <div className="pt-6 border-t border-slate-50">
                        <button
                            onClick={() => router.push('/login')}
                            className="inline-flex items-center gap-3 text-slate-400 font-black text-xs uppercase tracking-widest hover:text-teal-600 transition-colors group"
                        >
                            <ArrowRight className="h-5 w-5 rotate-180 group-hover:-translate-x-1 transition-transform" />
                            Back to Secure Login
                        </button>
                    </div>
                </motion.div>

                <div className="mt-12 text-center">
                    <p className="text-[10px] font-black uppercase tracking-[0.4em] text-slate-300 opacity-60">
                        Secure Cryptographic Verification • One-Time Code
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
