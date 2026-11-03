"use client";

import { useState, useEffect, useRef } from "react"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Loader2, ArrowRight, KeyRound, Sparkles, Eye, EyeOff, CheckCircle2, Circle, Lock, AlertCircle } from "lucide-react"
import { useRouter } from "next/navigation"
import Link from "next/link"
import { supaSetNewPassword, supaSession, SupabaseNotConfiguredError } from "@/lib/supabase"
import { motion } from "framer-motion"
import { cn } from "@/lib/utils"

export default function ResetPassword() {
    const router = useRouter()
    const [isLoading, setIsLoading] = useState(false)
    const [error, setError] = useState<string | null>(null)
    const [passwordError, setPasswordError] = useState<string | null>(null)
    const [confirmError, setConfirmError] = useState<string | null>(null)
    const [success, setSuccess] = useState(false)
    const [password, setPassword] = useState("")
    const [confirmPassword, setConfirmPassword] = useState("")
    const [showPassword, setShowPassword] = useState(false)
    const [linkValid, setLinkValid] = useState(false)
    const passwordRef = useRef<HTMLInputElement>(null)
    const confirmRef = useRef<HTMLInputElement>(null)
    const errorBannerRef = useRef<HTMLDivElement>(null)
    const successRef = useRef<HTMLDivElement>(null)

    // On load, check whether a live Supabase recovery session exists
    // (detectSessionInUrl applied the pending recovery token from the
    // password-reset link).
    useEffect(() => {
        supaSession()
            .then((info) => {
                if (info) {
                    setLinkValid(true)
                } else {
                    setError("Invalid or expired reset link. Please request a new one from /forgot-password.")
                }
            })
            .catch(() => {
                setError("Failed to read the reset link. Please request a new one from /forgot-password.")
            })
    }, [])

    useEffect(() => {
        if (success) successRef.current?.focus()
    }, [success])

    const checklist = [
        { id: "len", ok: password.length >= 8, label: "At least 8 characters" },
        { id: "lower", ok: /[a-z]/.test(password), label: "One lowercase letter" },
        { id: "upper", ok: /[A-Z]/.test(password), label: "One uppercase letter" },
        { id: "digit", ok: /\d/.test(password), label: "One number" },
    ]

    const handleSubmit = async (e: React.FormEvent) => {
        e.preventDefault()
        setError(null)
        setPasswordError(null)
        setConfirmError(null)

        const pwIssues: string[] = []
        if (password.length < 8) pwIssues.push("at least 8 characters")
        if (!/[a-z]/.test(password)) pwIssues.push("one lowercase letter")
        if (!/[A-Z]/.test(password)) pwIssues.push("one uppercase letter")
        if (!/\d/.test(password)) pwIssues.push("one number")
        if (pwIssues.length > 0) {
            setPasswordError(`Password needs ${pwIssues.join(", ")}.`)
        }

        let hasConfirmError = false
        if (!confirmPassword) {
            setConfirmError("Please confirm your new password.")
            hasConfirmError = true
        } else if (confirmPassword !== password) {
            setConfirmError("Passwords do not match.")
            hasConfirmError = true
        }

        if (pwIssues.length > 0 || hasConfirmError) {
            if (pwIssues.length > 0) passwordRef.current?.focus()
            else confirmRef.current?.focus()
            return
        }

        setIsLoading(true)
        try {
            // The Supabase client applied the pending recovery token from the
            // URL on load (detectSessionInUrl). updateUser({password}) posts
            // the new password against that token.
            await supaSetNewPassword(password)

            setSuccess(true)
            setTimeout(() => {
                router.push('/login')
            }, 3000)
        } catch (err: unknown) {
            // Preserve the typed passwords on a recoverable server error.
            setError(
                err instanceof SupabaseNotConfiguredError
                    ? "Password reset is unavailable right now. Please try again shortly."
                    : "Failed to reset password. The link may have expired."
            )
            errorBannerRef.current?.focus()
        } finally {
            setIsLoading(false)
        }
    }

    return (
        <div className="min-h-screen flex items-center justify-center bg-[#FDFDFF] text-slate-700 font-sans relative overflow-hidden">
            {/* Background Aesthetics */}
            <div className="absolute inset-0 bg-[radial-gradient(circle_at_50%_50%,#f0f9ff_0%,transparent_100%)] opacity-50"></div>

            <div className="w-full max-w-lg px-8 relative z-10">
                <motion.div
                    initial={{ opacity: 0, y: 20 }}
                    animate={{ opacity: 1, y: 0 }}
                    className="bg-white rounded-[3rem] shadow-skeuo-floating border border-slate-50 p-12 text-center space-y-10"
                >
                    {/* Header */}
                    <div className="space-y-4">
                        <div className="mx-auto w-24 h-24 rounded-3xl bg-white shadow-skeuo-md flex items-center justify-center mb-6 border-2 border-white">
                            <Lock className="h-10 w-10 text-teal-600" />
                        </div>
                        <div className="flex items-center justify-center gap-2">
                            <Sparkles className="h-5 w-5 text-teal-500" />
                            <span className="text-xl font-black text-slate-800 tracking-tighter uppercase">HealthTrack+ Elite</span>
                        </div>
                        <h2 className="text-4xl font-black text-slate-800 tracking-tight">Set Security Key</h2>
                        <p className="text-slate-400 font-bold text-lg mt-2 leading-relaxed">
                            {success
                                ? "Identity restored. Synchronizing your profile."
                                : "Define a new cryptographic entry key for your account."}
                        </p>
                    </div>

                    {success ? (
                        <div className="space-y-8">
                            <motion.div
                                ref={successRef}
                                tabIndex={-1}
                                initial={{ scale: 0.9, opacity: 0 }}
                                animate={{ scale: 1, opacity: 1 }}
                                className="p-8 rounded-[2.5rem] bg-emerald-50 border border-emerald-100 flex flex-col items-center gap-4 text-center outline-none"
                            >
                                <div className="h-16 w-16 rounded-2xl bg-white shadow-sm flex items-center justify-center">
                                    <CheckCircle2 className="h-10 w-10 text-emerald-500" />
                                </div>
                                <div>
                                    <p className="text-slate-800 font-black text-xl mb-1">Key Synchronized</p>
                                    <p className="text-slate-500 font-medium">Redirecting to login portal...</p>
                                </div>
                            </motion.div>
                        </div>
                    ) : (
                        <form onSubmit={handleSubmit} noValidate className="space-y-8 text-left">
                            {error && (
                                <motion.div
                                    ref={errorBannerRef}
                                    role="alert"
                                    tabIndex={-1}
                                    initial={{ opacity: 0, scale: 0.95 }}
                                    animate={{ opacity: 1, scale: 1 }}
                                    className="p-5 rounded-3xl bg-red-50 text-red-600 border border-red-100 flex items-start gap-4 outline-none"
                                >
                                    <AlertCircle className="h-6 w-6 mt-0.5 shrink-0" />
                                    <p className="text-base font-bold leading-tight">{error}</p>
                                </motion.div>
                            )}

                            <div className="space-y-6">
                                <div className="space-y-3">
                                    <Label htmlFor="password" className="text-xs font-black text-slate-400 uppercase tracking-[0.2em] ml-2">New Security Key</Label>
                                    <div className="relative group">
                                        <div className={cn("absolute left-5 top-1/2 -translate-y-1/2 transition-colors duration-300", password ? "text-teal-600" : "text-slate-300")}>
                                            <KeyRound className="h-6 w-6" />
                                        </div>
                                        <Input
                                            ref={passwordRef}
                                            id="password"
                                            type={showPassword ? "text" : "password"}
                                            required
                                            disabled={!linkValid}
                                            aria-invalid={passwordError ? true : undefined}
                                            aria-describedby={passwordError ? "password-error" : "password-checklist"}
                                            className={cn(
                                                "h-16 pl-14 pr-14 bg-slate-50/50 border-transparent rounded-2xl focus:bg-white focus:ring-[6px] focus:ring-slate-100 focus:border-slate-200 transition-all text-lg font-medium shadow-none",
                                                passwordError && "border-red-200 focus:ring-red-100"
                                            )}
                                            value={password}
                                            onChange={(e) => { setPassword(e.target.value); setPasswordError(null) }}
                                            autoComplete="new-password"
                                        />
                                        <button
                                            type="button"
                                            onClick={() => setShowPassword(!showPassword)}
                                            aria-label={showPassword ? "Hide password" : "Show password"}
                                            className="absolute right-5 top-1/2 -translate-y-1/2 text-slate-300 hover:text-slate-500 transition-colors"
                                        >
                                            {showPassword ? <EyeOff className="h-6 w-6" /> : <Eye className="h-6 w-6" />}
                                        </button>
                                    </div>
                                    {passwordError && (
                                        <p id="password-error" role="alert" className="text-sm font-bold text-red-500 ml-2">{passwordError}</p>
                                    )}
                                    {/* Live password requirements checklist */}
                                    <ul id="password-checklist" className={cn("space-y-1.5 ml-2", password ? "block" : "sr-only")}>
                                        {checklist.map((item) => (
                                            <li key={item.id} className={cn("flex items-center gap-2 text-xs font-bold", item.ok ? "text-emerald-500" : "text-slate-300")}>
                                                {item.ok ? <CheckCircle2 className="h-4 w-4" /> : <Circle className="h-4 w-4" />}
                                                {item.label}
                                            </li>
                                        ))}
                                    </ul>
                                </div>

                                <div className="space-y-3">
                                    <Label htmlFor="confirm" className="text-xs font-black text-slate-400 uppercase tracking-[0.2em] ml-2">Confirm Key</Label>
                                    <div className="relative group">
                                        <div className={cn("absolute left-5 top-1/2 -translate-y-1/2 transition-colors duration-300", confirmPassword === password && password ? "text-teal-600" : "text-slate-300")}>
                                            <KeyRound className="h-6 w-6" />
                                        </div>
                                        <Input
                                            ref={confirmRef}
                                            id="confirm"
                                            name="confirmPassword"
                                            type="password"
                                            required
                                            disabled={!linkValid}
                                            aria-invalid={confirmError ? true : undefined}
                                            aria-describedby={confirmError ? "confirm-error" : undefined}
                                            className={cn(
                                                "h-16 pl-14 pr-6 bg-slate-50/50 border-transparent rounded-2xl focus:bg-white focus:ring-[6px] focus:ring-slate-100 focus:border-slate-200 transition-all text-lg font-medium shadow-none",
                                                confirmError && "border-red-200 focus:ring-red-100"
                                            )}
                                            value={confirmPassword}
                                            onChange={(e) => { setConfirmPassword(e.target.value); setConfirmError(null) }}
                                            autoComplete="new-password"
                                        />
                                    </div>
                                    {confirmError && (
                                        <p id="confirm-error" role="alert" className="text-sm font-bold text-red-500 ml-2">{confirmError}</p>
                                    )}
                                </div>
                            </div>

                            <Button
                                type="submit"
                                aria-busy={isLoading || undefined}
                                className="w-full h-18 bg-teal-600 text-white font-black text-xl rounded-[2rem] shadow-2xl shadow-teal-500/20 transition-all hover:scale-[1.02] active:scale-[0.98] disabled:opacity-70 group"
                                disabled={isLoading || !linkValid}
                            >
                                {isLoading ? (
                                    <span className="flex items-center justify-center gap-3">
                                        <Loader2 className="h-8 w-8 animate-spin" /> Finalizing
                                    </span>
                                ) : (
                                    <span className="flex items-center justify-center gap-3">
                                        Finalize Reset <ArrowRight className="h-6 w-6 group-hover:translate-x-2 transition-transform" />
                                    </span>
                                )}
                            </Button>
                        </form>
                    )}

                    <div className="pt-6 border-t border-slate-50 text-center">
                        <Link
                            href="/login"
                            className="text-xs font-black uppercase tracking-widest text-slate-300 hover:text-teal-600 transition-colors"
                        >
                            Return to Entry Portal
                        </Link>
                    </div>
                </motion.div>

                <div className="mt-12 text-center">
                    <p className="text-[10px] font-black uppercase tracking-[0.4em] text-slate-300 opacity-60">
                        Institutional Recovery System • 256-BIT ENCRYPTED
                    </p>
                </div>
            </div>
        </div>
    )
}
