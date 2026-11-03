"use client";

import { useState, useEffect, Suspense, useRef } from "react"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Loader2, User, Mail, Lock, Eye, EyeOff, ArrowRight, Sparkles, ShieldCheck, Zap, Stethoscope, Building2, AlertCircle, RefreshCw } from "lucide-react"
import Link from "next/link"
import { useRouter, useSearchParams } from "next/navigation"
import { cn, syncSessionFromSupabase } from "@/lib/utils"
import {
    supaSignIn,
    supaGoogleSignIn,
    supaResendVerification,
    AuthError,
    SupabaseNotConfiguredError,
    roleOfSupaUser,
    type SupaSessionInfo,
} from "@/lib/supabase"
import { apiProfile } from "@/lib/api"
import { roleHomePath } from "@/components/Providers"
import { motion, AnimatePresence } from "framer-motion"

// The four-color Google "G" glyph for the native sign-in button (we no longer
// use @react-oauth/google; Google auth goes through Supabase OAuth).
function GoogleGlyph() {
    return (
        <svg className="h-5 w-5 shrink-0" viewBox="0 0 24 24" aria-hidden="true" focusable="false">
            <path fill="#4285F4" d="M12.24 10.24v3.58h5.68c-.24 1.32-1.02 2.44-2.2 3.18v2.62h3.56c2.08-1.82 3.28-4.5 3.28-7.68 0-.6-.06-1.18-.16-1.74H12.24z" />
            <path fill="#34A853" d="M18.42 17.04l-3.56 2.62v2.62c2.08-1.06 3.64-2.58 4.56-4.52z" />
            <path fill="#FBBC05" d="M12.24 22.3a8.5 8.5 0 0 1-4.96-1.62l-3.36 2.56A11.66 11.66 0 0 0 12.24 24z" />
            <path fill="#EA4335" d="M7.28 12.24V8.98l-3.36 2.56zM12.24 7.22c1.48 0 2.8.52 3.84 1.48l2.9-2.9C17.5 4.5 15.08 3.3 12.24 3.3 8.9 3.3 5.9 5.1 4.26 7.86l3.36 2.56c.78-1.9 2.56-3.2 4.62-3.2z" />
        </svg>
    )
}

function LoginForm() {
    const router = useRouter()
    const searchParams = useSearchParams()
    const [isLoading, setIsLoading] = useState(false)
    const [showPassword, setShowPassword] = useState(false)
    const [error, setError] = useState<string | null>(null)
    const [fieldErrors, setFieldErrors] = useState<{ username?: string; password?: string }>({})
    const [needsVerification, setNeedsVerification] = useState(false)
    const [unverifiedEmail, setUnverifiedEmail] = useState("")
    const [resendStatus, setResendStatus] = useState<'idle' | 'loading' | 'success'>('idle')
    const googlePending = useRef(false)
    const usernameRef = useRef<HTMLInputElement>(null)
    const passwordRef = useRef<HTMLInputElement>(null)
    
    const [formData, setFormData] = useState({
        username: "",
        password: "",
        role: "patient"
    })

    useEffect(() => {
        const roleParam = searchParams.get('role');
        if (roleParam && ['patient', 'doctor', 'provider'].includes(roleParam)) {
            setFormData(prev => ({ ...prev, role: roleParam }));
        }
    }, [searchParams])

    const applyAuth = async (info: SupaSessionInfo) => {
        // Resolve the app role: Supabase user_metadata first, then the Go
        // profile (best-effort), defaulting to 'patient'.
        let role = roleOfSupaUser(info.user) ?? ""
        if (!role) {
            try {
                // Temporarily publish the token so apiProfile's Bearer header
                // is set, then read the profile's role.
                syncSessionFromSupabase(info.accessToken, null)
                const p = await apiProfile()
                role = (p?.user?.role as string) ?? "patient"
            } catch {
                role = "patient"
            }
        }
        syncSessionFromSupabase(info.accessToken, role)
        router.push(roleHomePath(role))
    }

    const errorBannerRef = useRef<HTMLDivElement>(null)

    const validate = () => {
        const next: { username?: string; password?: string } = {}
        if (!formData.username.trim()) next.username = "Please enter your username or email."
        if (!formData.password) next.password = "Please enter your password."
        setFieldErrors(next)
        if (next.username) {
            usernameRef.current?.focus()
            return false
        }
        if (next.password) {
            passwordRef.current?.focus()
            return false
        }
        return true
    }

    const handleSubmit = async (e: React.FormEvent) => {
        e.preventDefault()
        setError(null)
        setNeedsVerification(false)

        if (!validate()) return

        // Supabase Auth signs in by email; a plain username (no '@') can't be
        // used with the email-provider.
        const identifier = formData.username.trim()
        if (!identifier.includes("@")) {
            setError("Please sign in with your email address.")
            errorBannerRef.current?.focus()
            return
        }

        setIsLoading(true)

        try {
            const info = await supaSignIn(identifier, formData.password)
            await applyAuth(info)
        } catch (err: unknown) {
            let nextError: string
            if (err instanceof AuthError && (err.code === "email_not_confirmed" || /not confirmed/i.test(err.message))) {
                setNeedsVerification(true)
                setUnverifiedEmail(identifier)
                nextError = "Your email address requires verification. Check your inbox for the confirmation link."
            } else if (err instanceof AuthError) {
                nextError = /invalid/i.test(err.message)
                    ? "Incorrect email or password. Please try again."
                    : err.message
            } else if (err instanceof SupabaseNotConfiguredError) {
                nextError = "Sign-in is unavailable right now (auth not configured). Please try again shortly."
            } else {
                nextError = "Connection failed. Please check your internet."
            }
            setError(nextError)
            // Focus the global error banner so screen readers announce it.
            errorBannerRef.current?.focus()
        } finally {
            setIsLoading(false)
        }
    }

    const handleResendVerification = async () => {
        setResendStatus('loading')
        try {
            await supaResendVerification(unverifiedEmail)
            setResendStatus('success')
            setTimeout(() => setResendStatus('idle'), 5000)
        } catch (err: unknown) {
            setError(err instanceof SupabaseNotConfiguredError
                ? "Verification resend is unavailable right now. Please try again shortly."
                : "Could not send the verification email. Please try again.")
            setResendStatus('idle')
        }
    }

    const handleChange = (e: React.ChangeEvent<HTMLInputElement>) => {
        setFormData(prev => ({ ...prev, [e.target.name]: e.target.value }))
        // Clear that field's inline error as the user fixes it.
        setFieldErrors(prev => ({ ...prev, [e.target.name]: undefined }))
    }

    const roleStyles = {
        patient: {
            bg: "bg-teal-600",
            lightBg: "bg-teal-50",
            text: "text-teal-600",
            shadow: "shadow-teal-500/20",
            icon: <User className="h-4 w-4" />,
            title: "Patient Portal",
            desc: "Access your medical history and AI health insights.",
            visualTitle: "Smart health tracking.",
            accent: "text-teal-500"
        },
        doctor: {
            bg: "bg-blue-600",
            lightBg: "bg-blue-50",
            text: "text-blue-600",
            shadow: "shadow-blue-500/20",
            icon: <Stethoscope className="h-4 w-4" />,
            title: "Doctor Console",
            desc: "Manage your patients, schedule, and prescriptions.",
            visualTitle: "Streamlined practice.",
            accent: "text-blue-500"
        },
        provider: {
            bg: "bg-indigo-600",
            lightBg: "bg-indigo-50",
            text: "text-indigo-600",
            shadow: "shadow-indigo-500/20",
            icon: <Building2 className="h-4 w-4" />,
            title: "Provider Network",
            desc: "Handle lab requests, pharmacy orders, and diagnostics.",
            visualTitle: "Network efficiency.",
            accent: "text-indigo-500"
        }
    }

    const currentStyle = roleStyles[formData.role as keyof typeof roleStyles]

    return (
        <div className="min-h-screen grid lg:grid-cols-2 bg-[#FDFDFF] text-slate-700 overflow-hidden font-sans">
            {/* Visual Side (Left) */}
            <AnimatePresence mode="wait">
                <motion.div 
                    key={formData.role}
                    initial={{ opacity: 0, x: -20 }}
                    animate={{ opacity: 1, x: 0 }}
                    exit={{ opacity: 0, x: 20 }}
                    transition={{ duration: 0.5, ease: "circOut" }}
                    className={cn("hidden lg:flex relative items-center justify-center p-16 overflow-hidden border-r border-slate-100", currentStyle.lightBg)}
                >
                    <div className="absolute inset-0 bg-[radial-gradient(circle_at_30%_30%,#fff_0%,transparent_70%)] opacity-60"></div>
                    
                    <div className="relative z-10 w-full max-w-lg space-y-12">
                        <div className="space-y-6">
                            <motion.div 
                                initial={{ opacity: 0, y: 10 }}
                                animate={{ opacity: 1, y: 0 }}
                                transition={{ delay: 0.2 }}
                                className="inline-flex items-center gap-2 px-4 py-2 rounded-full bg-white shadow-skeuo-sm border border-white"
                            >
                                <Sparkles className={cn("h-4 w-4", currentStyle.accent)} />
                                <span className={cn("text-xs font-black tracking-wider uppercase", currentStyle.text)}>HealthTrack+ Elite</span>
                            </motion.div>
                            
                            <h1 className="text-7xl font-black text-slate-800 leading-[1.05] tracking-tighter">
                                {currentStyle.visualTitle.split(' ')[0]} <br />
                                <span className={currentStyle.text}>{currentStyle.visualTitle.split(' ').slice(1).join(' ')}</span>
                            </h1>
                            <p className="text-xl text-slate-500 max-w-sm font-medium leading-relaxed">
                                {currentStyle.desc}
                            </p>
                        </div>

                        <div className="space-y-6">
                            {[
                                { icon: ShieldCheck, title: "Private & Encrypted", desc: "Your data travels over encrypted TLS connections and stays scoped to your account." },
                                { icon: Zap, title: "Real-time Sync", desc: "Instant updates across all your medical touchpoints." }
                            ].map((item, i) => (
                                <div key={i} className="flex gap-6 p-8 rounded-[2.5rem] bg-white/60 backdrop-blur-xl border border-white shadow-skeuo-md">
                                    <div className={cn("h-14 w-14 rounded-2xl bg-white shadow-skeuo-sm flex items-center justify-center shrink-0", currentStyle.text)}>
                                        <item.icon className="h-7 w-7" />
                                    </div>
                                    <div>
                                        <h3 className="font-bold text-slate-800 text-xl mb-1">{item.title}</h3>
                                        <p className="text-base text-slate-500 font-medium leading-normal">{item.desc}</p>
                                    </div>
                                </div>
                            ))}
                        </div>
                    </div>
                    
                    <div className={cn("absolute -bottom-24 -left-24 w-96 h-96 rounded-full border border-white/40 backdrop-blur-2xl bg-white/10 opacity-50")}></div>
                </motion.div>
            </AnimatePresence>

            {/* Content Side (Right) */}
            <div className="flex items-center justify-center p-8 sm:p-12 lg:p-24 relative bg-white">
                <div className="w-full max-w-md space-y-12">
                    {/* Header */}
                    <div className="space-y-4">
                        <div className="flex items-center gap-3 mb-4">
                            <motion.div 
                                animate={{ rotate: 360 }}
                                transition={{ duration: 20, repeat: Infinity, ease: "linear" }}
                                className={cn("h-12 w-12 bg-white rounded-2xl flex items-center justify-center shadow-skeuo-md border-2 border-white", currentStyle.text)}
                            >
                                <Sparkles className="h-6 w-6" />
                            </motion.div>
                            <span className="text-3xl font-black text-slate-800 tracking-tighter">HealthTrack+</span>
                        </div>
                        <AnimatePresence mode="wait">
                            <motion.div
                                key={formData.role}
                                initial={{ opacity: 0, y: 10 }}
                                animate={{ opacity: 1, y: 0 }}
                                exit={{ opacity: 0, y: -10 }}
                                transition={{ duration: 0.3 }}
                            >
                                <h2 className="text-5xl font-black text-slate-800 tracking-tight leading-tight">
                                    {currentStyle.title}
                                </h2>
                                <p className="text-slate-400 font-semibold text-lg mt-2">Sign in to your secure workspace.</p>
                            </motion.div>
                        </AnimatePresence>
                    </div>

                    {/* Role Selector - Premium Interaction */}
                    <div className="p-2 rounded-[2rem] bg-slate-50 shadow-inner-lg flex items-center gap-2 border border-slate-100">
                        {(['patient', 'doctor', 'provider'] as const).map((r) => (
                            <button
                                key={r}
                                type="button"
                                onClick={() => setFormData(prev => ({ ...prev, role: r }))}
                                className={cn(
                                    "flex-1 flex flex-col items-center justify-center gap-2 py-4 rounded-[1.5rem] text-xs font-black uppercase tracking-widest transition-all duration-500",
                                    formData.role === r
                                        ? cn("bg-white shadow-skeuo-floating scale-[1.05] border-2", roleStyles[r].text, "border-white")
                                        : "text-slate-300 hover:text-slate-500"
                                )}
                            >
                                <div className={cn(
                                    "p-2 rounded-lg transition-colors",
                                    formData.role === r ? roleStyles[r].lightBg : "bg-transparent"
                                )}>
                                    {roleStyles[r].icon}
                                </div>
                                {r}
                            </button>
                        ))}
                    </div>

                    {/* Login Form */}
                    <form onSubmit={handleSubmit} noValidate className="space-y-8">
                        {error && (
                            <motion.div
                                ref={errorBannerRef}
                                role="alert"
                                tabIndex={-1}
                                initial={{ opacity: 0, scale: 0.95 }}
                                animate={{ opacity: 1, scale: 1 }}
                                className={cn(
                                    "p-5 rounded-3xl flex items-start gap-4 outline-none",
                                    needsVerification ? "bg-amber-50 text-amber-700 border border-amber-100" : "bg-red-50 text-red-600 border border-red-100"
                                )}
                            >
                                <AlertCircle className="h-6 w-6 mt-0.5 shrink-0" />
                                <div className="space-y-3">
                                    <p className="text-base font-bold leading-tight">{error}</p>
                                    {needsVerification && (
                                        <button
                                            type="button"
                                            onClick={handleResendVerification}
                                            disabled={resendStatus === 'loading' || isLoading}
                                            className="flex items-center gap-2 text-xs font-black uppercase tracking-[0.15em] py-2 px-4 rounded-full bg-white shadow-sm hover:shadow-md transition-all active:scale-95"
                                        >
                                            {resendStatus === 'loading' ? <Loader2 className="h-3 w-3 animate-spin" /> : <RefreshCw className="h-3 w-3" />}
                                            {resendStatus === 'success' ? "Link Sent!" : "Resend Link"}
                                        </button>
                                    )}
                                </div>
                            </motion.div>
                        )}

                        <div className="space-y-6">
                            <div className="space-y-2">
                                <Label htmlFor="username" className="text-xs font-black text-slate-400 uppercase tracking-[0.2em] ml-2">Identify Yourself</Label>
                                <div className="relative group">
                                    <div className={cn("absolute left-5 top-1/2 -translate-y-1/2 transition-colors duration-300", formData.username ? currentStyle.text : "text-slate-300")}>
                                        <Mail className="h-6 w-6" />
                                    </div>
                                    <Input
                                        ref={usernameRef}
                                        id="username"
                                        name="username"
                                        required
                                        aria-invalid={fieldErrors.username ? true : undefined}
                                        aria-describedby={fieldErrors.username ? "username-error" : undefined}
                                        className={cn(
                                            "h-16 pl-14 pr-6 bg-slate-50/50 border-transparent rounded-2xl focus:bg-white focus:ring-[6px] focus:ring-slate-100 focus:border-slate-200 transition-all text-lg font-medium shadow-none",
                                            fieldErrors.username && "border-red-200 focus:ring-red-100"
                                        )}
                                        autoComplete="username"
                                        value={formData.username}
                                        onChange={handleChange}
                                    />
                                </div>
                                {fieldErrors.username && (
                                    <p id="username-error" role="alert" className="text-sm font-bold text-red-500 ml-2">
                                        {fieldErrors.username}
                                    </p>
                                )}
                            </div>

                            <div className="space-y-2">
                                <div className="flex justify-between items-center px-2">
                                    <Label htmlFor="password" className="text-xs font-black text-slate-400 uppercase tracking-[0.2em]">Security Key</Label>
                                    <Link href="/forgot-password" className="text-xs font-black text-slate-300 hover:text-teal-600 transition-colors uppercase tracking-widest">
                                        Forgot?
                                    </Link>
                                </div>
                                <div className="relative group">
                                    <div className={cn("absolute left-5 top-1/2 -translate-y-1/2 transition-colors duration-300", formData.password ? currentStyle.text : "text-slate-300")}>
                                        <Lock className="h-6 w-6" />
                                    </div>
                                    <Input
                                        ref={passwordRef}
                                        id="password"
                                        name="password"
                                        type={showPassword ? "text" : "password"}
                                        required
                                        aria-invalid={fieldErrors.password ? true : undefined}
                                        aria-describedby={fieldErrors.password ? "password-error" : undefined}
                                        className={cn(
                                            "h-16 pl-14 pr-14 bg-slate-50/50 border-transparent rounded-2xl focus:bg-white focus:ring-[6px] focus:ring-slate-100 focus:border-slate-200 transition-all text-lg font-medium shadow-none",
                                            fieldErrors.password && "border-red-200 focus:ring-red-100"
                                        )}
                                        autoComplete="current-password"
                                        value={formData.password}
                                        onChange={handleChange}
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
                                {fieldErrors.password && (
                                    <p id="password-error" role="alert" className="text-sm font-bold text-red-500 ml-2">
                                        {fieldErrors.password}
                                    </p>
                                )}
                            </div>
                        </div>

                        <Button
                            type="submit"
                            aria-busy={isLoading || undefined}
                            className={cn(
                                "w-full h-18 text-white font-black text-xl rounded-[2rem] shadow-2xl transition-all hover:scale-[1.02] active:scale-[0.98] disabled:opacity-70 group",
                                currentStyle.bg,
                                currentStyle.shadow
                            )}
                            disabled={isLoading}
                        >
                            {isLoading ? (
                                <span className="flex items-center justify-center gap-3">
                                    <Loader2 className="h-8 w-8 animate-spin" /> Authenticating
                                </span>
                            ) : (
                                <span className="flex items-center justify-center gap-3">
                                    Authenticate <ArrowRight className="h-6 w-6 group-hover:translate-x-2 transition-transform duration-300" />
                                </span>
                            )}
                        </Button>
                    </form>

                    <div className="space-y-10">
                        <div className="relative">
                            <div className="absolute inset-0 flex items-center">
                                <div className="w-full border-t border-slate-100"></div>
                            </div>
                            <div className="relative flex justify-center text-[10px] font-black uppercase tracking-[0.3em] text-slate-300">
                                <span className="bg-white px-6">Elite Access</span>
                            </div>
                        </div>

                        <div className="flex justify-center">
                            <button
                                type="button"
                                onClick={async () => {
                                    if (googlePending.current) return
                                    googlePending.current = true
                                    setError(null)
                                    setIsLoading(true)
                                    try {
                                        const { url } = await supaGoogleSignIn()
                                        if (url) window.location.assign(url)
                                    } catch (err: unknown) {
                                        setError(
                                            err instanceof SupabaseNotConfiguredError
                                                ? "Google sign-in is unavailable right now (auth not configured)."
                                                : "Google sign-in is not available for this account. Please use email instead."
                                        )
                                        errorBannerRef.current?.focus()
                                    } finally {
                                        // Only clear if we did not navigate away.
                                        if (document.visibilityState === "visible") {
                                            googlePending.current = false
                                            setIsLoading(false)
                                        }
                                    }
                                }}
                                disabled={isLoading}
                                className={cn(
                                    "flex items-center gap-3 px-6 py-3.5 rounded-full bg-white border border-slate-200 shadow-skeuo-sm transition-all hover:shadow-md active:scale-95 disabled:opacity-70",
                                    "text-slate-700 text-sm font-black uppercase tracking-widest"
                                )}
                                aria-label="Sign in with Google"
                            >
                                <GoogleGlyph />
                                Continue with Google
                            </button>
                        </div>

                        <div className="text-center pt-4">
                            <p className="text-slate-400 font-bold">
                                New to the network?{" "}
                                <Link href={`/register?role=${formData.role}`} className={cn("font-black hover:underline decoration-4 underline-offset-8 transition-colors", currentStyle.text)}>
                                    Join Now
                                </Link>
                            </p>
                        </div>
                    </div>
                </div>
            </div>
            
            {/* Custom Overlay for Role Changes */}
            <AnimatePresence>
                {isLoading && (
                    <motion.div 
                        initial={{ opacity: 0 }}
                        animate={{ opacity: 1 }}
                        exit={{ opacity: 0 }}
                        className="fixed inset-0 z-[100] bg-white/60 backdrop-blur-sm flex items-center justify-center"
                    >
                        <div className="flex flex-col items-center gap-6">
                            <div className={cn("w-20 h-20 rounded-3xl flex items-center justify-center animate-bounce shadow-2xl bg-white border-2 border-slate-100", currentStyle.text)}>
                                <Sparkles className="h-10 w-10" />
                            </div>
                            <p className="text-xs font-black tracking-[0.4em] uppercase text-slate-500 animate-pulse">Establishing Secure Session</p>
                        </div>
                    </motion.div>
                )}
            </AnimatePresence>
        </div>
    )
}

export default function Login() {
    return (
        <Suspense fallback={<div className="min-h-screen flex items-center justify-center bg-white font-black tracking-widest text-slate-300">SYSTEM_INIT...</div>}>
            <LoginForm />
        </Suspense>
    )
}
