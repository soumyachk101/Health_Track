"use client";

import { useState, useEffect, Suspense, useRef } from "react"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Loader2, User, Mail, Lock, Eye, EyeOff, ArrowRight, Sparkles, ShieldCheck, Zap, Stethoscope, Building2, AlertCircle, CheckCircle2, Circle, Phone, BadgeCheck, Briefcase } from "lucide-react"
import Link from "next/link"
import { useRouter, useSearchParams } from "next/navigation"
import { cn, syncSessionFromSupabase } from "@/lib/utils"
import {
    supaSignUp,
    SupabaseNotConfiguredError,
} from "@/lib/supabase"
import { apiRegisterProfile } from "@/lib/api"
import { roleHomePath } from "@/components/Providers"
import { motion, AnimatePresence } from "framer-motion"

function RegisterForm() {
    const router = useRouter()
    const searchParams = useSearchParams()
    const [isLoading, setIsLoading] = useState(false)
    const [showPassword, setShowPassword] = useState(false)
    const [error, setError] = useState<string | null>(null)
    const [fieldErrors, setFieldErrors] = useState<Record<string, string>>({})
    const [isSuccess, setIsSuccess] = useState(false)
    const errorBannerRef = useRef<HTMLDivElement>(null)
    const successRef = useRef<HTMLDivElement>(null)

    // Move focus to the success card when the account is created.
    useEffect(() => {
        if (isSuccess) successRef.current?.focus()
    }, [isSuccess])
    
    const [formData, setFormData] = useState({
        username: "",
        email: "",
        password: "",
        role: "patient",
        fullName: "",
        phoneNumber: "",
        // Doctor specific
        licenseNumber: "",
        specialization: "",
        // Provider specific
        businessName: "",
        serviceCategory: ""
    })

    useEffect(() => {
        const roleParam = searchParams.get('role');
        if (roleParam && ['patient', 'doctor', 'provider'].includes(roleParam)) {
            setFormData(prev => ({ ...prev, role: roleParam }));
        }
    }, [searchParams])

    const EMAIL_RE = /^[^\s@]+@[^\s@]+\.[^\s@]+$/
    // Basic phone pattern: digits, spaces, dashes, parens, leading +; 7-15 digits total.
    const PHONE_RE = /^\+?[\d\s\-()]{7,20}$/

    const validate = (): Record<string, string> => {
        const next: Record<string, string> = {}
        const name = formData.fullName.trim()
        if (!name) next.fullName = "Full legal name is required."
        else if (name.length < 2) next.fullName = "Please enter your full name."
        else if (name === name.toLowerCase() || name === name.toUpperCase())
            next.fullName = "Please enter your name in normal casing (e.g., Ada Lovelace)."

        const email = formData.email.trim()
        if (!email) next.email = "Email address is required."
        else if (!EMAIL_RE.test(email)) next.email = "Please enter a valid email address."

        const username = formData.username.trim()
        if (!username) next.username = "Username is required."
        else if (username.length < 3) next.username = "Username must be at least 3 characters."

        const phone = formData.phoneNumber.trim()
        if (!phone) next.phoneNumber = "Phone number is required."
        else if (!PHONE_RE.test(phone)) next.phoneNumber = "Please enter a valid phone number (7-15 digits)."

        const password = formData.password
        if (!password) next.password = "Password is required."
        else if (password.length < 8) next.password = "Password must be at least 8 characters."
        else if (password.length > 72) next.password = "Password must be 72 characters or fewer."

        if (formData.role === "doctor") {
            if (!formData.licenseNumber.trim()) next.licenseNumber = "Medical license number is required."
            if (!formData.specialization.trim()) next.specialization = "Primary specialization is required."
        }
        if (formData.role === "provider") {
            if (!formData.businessName.trim()) next.businessName = "Legal business name is required."
            if (!formData.serviceCategory.trim()) next.serviceCategory = "Service category is required."
        }

        return next
    }

    const passwordChecks = [
        { id: "pw-len", ok: formData.password.length >= 8, label: "At least 8 characters" },
        { id: "pw-upper", ok: /[A-Z]/.test(formData.password), label: "One uppercase letter" },
        { id: "pw-lower", ok: /[a-z]/.test(formData.password), label: "One lowercase letter" },
        { id: "pw-digit", ok: /\d/.test(formData.password), label: "One number" },
    ]
    const passwordStrength = passwordChecks.filter((c) => c.ok).length

    const handleSubmit = async (e: React.FormEvent) => {
        e.preventDefault()
        setError(null)

        const nextErrors = validate()
        setFieldErrors(nextErrors)
        if (Object.keys(nextErrors).length > 0) {
            // Focus the first invalid field (fieldName -> DOM id is 1:1).
            const firstKey = Object.keys(nextErrors)[0]
            document.getElementById(firstKey)?.focus()
            return
        }

        setIsLoading(true)

        const nameParts = formData.fullName.trim().split(/\s+/)
        const providerType =
            formData.role === 'doctor' ? 'doctor'
            : formData.role === 'provider' ? (formData.serviceCategory || 'pharmacy')
            : ''

        // Supabase is the identity source of truth: the account + all
        // provisioning fields land in the Supabase user_metadata.
        try {
            const { session, user } = await supaSignUp({
                email: formData.email.trim(),
                password: formData.password,
                options: {
                    data: {
                        role: formData.role,
                        provider_type: providerType,
                        business_name: formData.businessName,
                        license_number: formData.licenseNumber,
                        specialization: formData.specialization,
                        first_name: nameParts[0] || "",
                        last_name: nameParts.slice(1).join(' ') || "",
                        username: formData.username.trim(),
                        phone: formData.phoneNumber.trim(),
                    },
                },
            })

            // Best-effort: mirror the user into the Go backend's profiles
            // table with the app-specific fields. A 404 (endpoint not
            // deployed yet) logs and continues — the trigger-based profile
            // row already exists server-side.
            await apiRegisterProfile({
                email: formData.email.trim(),
                role: formData.role,
                provider_type: providerType,
                business_name: formData.businessName,
                license_number: formData.licenseNumber,
                specialization: formData.specialization,
                first_name: nameParts[0] || "",
                last_name: nameParts.slice(1).join(' ') || "",
            })

            if (session && user) {
                // Email confirmation is OFF for this Supabase project: the
                // account is live right now. Log straight in and route home.
                const role = formData.role
                syncSessionFromSupabase(session.access_token, role)
                router.push(roleHomePath(role))
            } else {
                // Confirmation ON: no session yet; show the inbox state.
                setIsSuccess(true)
            }
        } catch (err: unknown) {
            const message =
                err instanceof SupabaseNotConfiguredError
                    ? "Account creation is unavailable right now (auth not configured). Please try again shortly."
                    : err instanceof Error
                        ? (err.message || "Could not create your account. Please try again.")
                        : "Could not create your account. Please try again."
            // Surface Supabase's friendly errors (e.g. "User already registered",
            // "Password should be at least 6 characters") instead of crashing.
            setError(message)
            // Preserve the user's input on a recoverable error.
            errorBannerRef.current?.focus()
        } finally {
            setIsLoading(false)
        }
    }

    const handleChange = (e: React.ChangeEvent<HTMLInputElement | HTMLSelectElement>) => {
        setFormData(prev => ({ ...prev, [e.target.name]: e.target.value }))
        // Clear that field's inline error as the user fixes it.
        setFieldErrors(prev => {
            if (!(e.target.name in prev)) return prev
            const next = { ...prev }
            delete next[e.target.name]
            return next
        })
    }

    const roleStyles = {
        patient: {
            bg: "bg-teal-600",
            lightBg: "bg-teal-50",
            text: "text-teal-600",
            border: "border-teal-200",
            accent: "text-teal-500",
            icon: <User className="h-4 w-4" />,
            title: "Join as Patient",
            desc: "Start your journey to better health with AI insights.",
            visualTitle: "Future of Wellness.",
            fields: []
        },
        doctor: {
            bg: "bg-blue-600",
            lightBg: "bg-blue-50",
            text: "text-blue-600",
            border: "border-blue-200",
            accent: "text-blue-500",
            icon: <Stethoscope className="h-4 w-4" />,
            title: "Doctor Registration",
            desc: "Onboard your practice and reach more patients.",
            visualTitle: "Expert Precision.",
            fields: [
                { name: "licenseNumber", label: "Medical License #", icon: BadgeCheck, type: "text" },
                { name: "specialization", label: "Primary Specialization", icon: Briefcase, type: "text" }
            ]
        },
        provider: {
            bg: "bg-indigo-600",
            lightBg: "bg-indigo-50",
            text: "text-indigo-600",
            border: "border-indigo-200",
            accent: "text-indigo-500",
            icon: <Building2 className="h-4 w-4" />,
            title: "Service Partner",
            desc: "Connect your lab or pharmacy to our digital network.",
            visualTitle: "Network Power.",
            fields: [
                { name: "businessName", label: "Legal Business Name", icon: Building2, type: "text" },
                { name: "serviceCategory", label: "Service Category", icon: Zap, type: "text" }
            ]
        }
    }

    const currentStyle = roleStyles[formData.role as keyof typeof roleStyles]

    if (isSuccess) {
        return (
            <div className="min-h-screen flex items-center justify-center bg-white p-6 font-sans">
                <motion.div
                    initial={{ scale: 0.9, opacity: 0 }}
                    animate={{ scale: 1, opacity: 1 }}
                    ref={successRef}
                    tabIndex={-1}
                    className="max-w-md w-full bg-white rounded-[3rem] shadow-skeuo-floating p-12 text-center space-y-8 border-4 border-slate-50 outline-none"
                >
                    <div className={cn("h-24 w-24 rounded-3xl mx-auto flex items-center justify-center shadow-lg animate-pulse bg-white", currentStyle.text)}>
                        <CheckCircle2 className="h-16 w-16" />
                    </div>
                    <div className="space-y-4">
                        <h2 className="text-4xl font-black text-slate-800 tracking-tight">Check Your Inbox</h2>
                        <p className="text-lg text-slate-500 font-medium leading-relaxed">
                            We&apos;ve sent a verification link to <span className="font-bold text-slate-800">{formData.email}</span>. Please verify to activate your {formData.role} account.
                        </p>
                    </div>
                    <Button 
                        onClick={() => router.push('/login')}
                        className={cn("w-full h-16 text-white font-black text-lg rounded-2xl transition-all shadow-xl", currentStyle.bg)}
                    >
                        Return to Login
                    </Button>
                </motion.div>
            </div>
        )
    }

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
                    className={cn("hidden lg:flex relative flex-col items-center justify-center p-16 overflow-hidden border-r border-slate-100", currentStyle.lightBg)}
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

                        <div className="grid grid-cols-1 gap-6">
                            {[
                                { icon: ShieldCheck, title: "Privacy First", desc: "Data minimization, you own your records, and no third-party ad tracking." },
                                { icon: Zap, title: "Instant Activation", desc: "Get verified and live in minutes, not days." }
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
                </motion.div>
            </AnimatePresence>

            {/* Content Side (Right) - Scrolling Form */}
            <div className="flex flex-col items-center justify-start p-8 sm:p-12 lg:p-16 relative bg-white overflow-y-auto scrollbar-thin">
                <div className="w-full max-w-xl space-y-12 my-auto py-12">
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
                                <p className="text-slate-400 font-semibold text-lg mt-2">Create your professional profile today.</p>
                            </motion.div>
                        </AnimatePresence>
                    </div>

                    {/* Role Selector */}
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

                    {/* Register Form */}
                    <form onSubmit={handleSubmit} noValidate className="space-y-10">
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

                        <div className="grid grid-cols-1 md:grid-cols-2 gap-8">
                            {/* Common Fields */}
                            <div className="space-y-2">
                                <Label htmlFor="fullName" className="text-xs font-black text-slate-400 uppercase tracking-[0.2em] ml-2">Full Legal Name</Label>
                                <div className="relative">
                                    <div className={cn("absolute left-5 top-1/2 -translate-y-1/2 transition-colors duration-300", formData.fullName ? currentStyle.text : "text-slate-300")}>
                                        <User className="h-6 w-6" />
                                    </div>
                                    <Input
                                        id="fullName"
                                        name="fullName"
                                        required
                                        aria-invalid={fieldErrors.fullName ? true : undefined}
                                        aria-describedby={fieldErrors.fullName ? "fullName-error" : undefined}
                                        className={cn(
                                            "h-16 pl-14 pr-6 bg-slate-50/50 border-transparent rounded-2xl focus:bg-white focus:ring-[6px] focus:ring-slate-100 focus:border-slate-200 transition-all text-lg font-medium shadow-none",
                                            fieldErrors.fullName && "border-red-200 focus:ring-red-100"
                                        )}
                                        autoComplete="name"
                                        value={formData.fullName}
                                        onChange={handleChange}
                                    />
                                </div>
                                {fieldErrors.fullName && (
                                    <p id="fullName-error" role="alert" className="text-sm font-bold text-red-500 ml-2">{fieldErrors.fullName}</p>
                                )}
                            </div>

                            <div className="space-y-2">
                                <Label htmlFor="phoneNumber" className="text-xs font-black text-slate-400 uppercase tracking-[0.2em] ml-2">Direct Phone</Label>
                                <div className="relative">
                                    <div className={cn("absolute left-5 top-1/2 -translate-y-1/2 transition-colors duration-300", formData.phoneNumber ? currentStyle.text : "text-slate-300")}>
                                        <Phone className="h-6 w-6" />
                                    </div>
                                    <Input
                                        id="phoneNumber"
                                        name="phoneNumber"
                                        type="tel"
                                        required
                                        aria-invalid={fieldErrors.phoneNumber ? true : undefined}
                                        aria-describedby={fieldErrors.phoneNumber ? "phoneNumber-error" : undefined}
                                        className={cn(
                                            "h-16 pl-14 pr-6 bg-slate-50/50 border-transparent rounded-2xl focus:bg-white focus:ring-[6px] focus:ring-slate-100 focus:border-slate-200 transition-all text-lg font-medium shadow-none",
                                            fieldErrors.phoneNumber && "border-red-200 focus:ring-red-100"
                                        )}
                                        autoComplete="tel"
                                        value={formData.phoneNumber}
                                        onChange={handleChange}
                                    />
                                </div>
                                {fieldErrors.phoneNumber && (
                                    <p id="phoneNumber-error" role="alert" className="text-sm font-bold text-red-500 ml-2">{fieldErrors.phoneNumber}</p>
                                )}
                            </div>

                            <div className="space-y-2">
                                <Label htmlFor="username" className="text-xs font-black text-slate-400 uppercase tracking-[0.2em] ml-2">Handle / Username</Label>
                                <div className="relative">
                                    <div className={cn("absolute left-5 top-1/2 -translate-y-1/2 transition-colors duration-300", formData.username ? currentStyle.text : "text-slate-300")}>
                                        <BadgeCheck className="h-6 w-6" />
                                    </div>
                                    <Input
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
                                    <p id="username-error" role="alert" className="text-sm font-bold text-red-500 ml-2">{fieldErrors.username}</p>
                                )}
                            </div>

                            <div className="space-y-2">
                                <Label htmlFor="email" className="text-xs font-black text-slate-400 uppercase tracking-[0.2em] ml-2">Email Address</Label>
                                <div className="relative">
                                    <div className={cn("absolute left-5 top-1/2 -translate-y-1/2 transition-colors duration-300", formData.email ? currentStyle.text : "text-slate-300")}>
                                        <Mail className="h-6 w-6" />
                                    </div>
                                    <Input
                                        id="email"
                                        name="email"
                                        type="email"
                                        required
                                        aria-invalid={fieldErrors.email ? true : undefined}
                                        aria-describedby={fieldErrors.email ? "email-error" : undefined}
                                        className={cn(
                                            "h-16 pl-14 pr-6 bg-slate-50/50 border-transparent rounded-2xl focus:bg-white focus:ring-[6px] focus:ring-slate-100 focus:border-slate-200 transition-all text-lg font-medium shadow-none",
                                            fieldErrors.email && "border-red-200 focus:ring-red-100"
                                        )}
                                        autoComplete="email"
                                        value={formData.email}
                                        onChange={handleChange}
                                    />
                                </div>
                                {fieldErrors.email && (
                                    <p id="email-error" role="alert" className="text-sm font-bold text-red-500 ml-2">{fieldErrors.email}</p>
                                )}
                            </div>

                            {/* Dynamic Fields Based on Role */}
                            <AnimatePresence mode="popLayout">
                                {currentStyle.fields.map((field) => (
                                    <motion.div
                                        key={field.name}
                                        initial={{ opacity: 0, scale: 0.95 }}
                                        animate={{ opacity: 1, scale: 1 }}
                                        exit={{ opacity: 0, scale: 0.95 }}
                                        className="space-y-2"
                                    >
                                        <Label htmlFor={field.name} className="text-xs font-black text-slate-400 uppercase tracking-[0.2em] ml-2">{field.label}</Label>
                                        <div className="relative">
                                            <div className={cn("absolute left-5 top-1/2 -translate-y-1/2 transition-colors duration-300", formData[field.name as keyof typeof formData] ? currentStyle.text : "text-slate-300")}>
                                                <field.icon className="h-6 w-6" />
                                            </div>
                                            <Input
                                                id={field.name}
                                                name={field.name}
                                                required
                                                aria-invalid={fieldErrors[field.name] ? true : undefined}
                                                aria-describedby={fieldErrors[field.name] ? `${field.name}-error` : undefined}
                                                autoComplete={field.name === "businessName" ? "organization" : "off"}
                                                className={cn(
                                                    "h-16 pl-14 pr-6 bg-slate-50/50 border-transparent rounded-2xl focus:bg-white focus:ring-[6px] focus:ring-slate-100 focus:border-slate-200 transition-all text-lg font-medium shadow-none",
                                                    fieldErrors[field.name] && "border-red-200 focus:ring-red-100"
                                                )}
                                                value={formData[field.name as keyof typeof formData]}
                                                onChange={handleChange}
                                            />
                                        </div>
                                        {fieldErrors[field.name] && (
                                            <p id={`${field.name}-error`} role="alert" className="text-sm font-bold text-red-500 ml-2">{fieldErrors[field.name]}</p>
                                        )}
                                    </motion.div>
                                ))}
                            </AnimatePresence>

                            <div className="space-y-2 md:col-span-2">
                                <Label htmlFor="password" className="text-xs font-black text-slate-400 uppercase tracking-[0.2em] ml-2">Create Security Key</Label>
                                <div className="relative">
                                    <div className={cn("absolute left-5 top-1/2 -translate-y-1/2 transition-colors duration-300", formData.password ? currentStyle.text : "text-slate-300")}>
                                        <Lock className="h-6 w-6" />
                                    </div>
                                    <Input
                                        id="password"
                                        name="password"
                                        type={showPassword ? "text" : "password"}
                                        required
                                        aria-invalid={fieldErrors.password ? true : undefined}
                                        aria-describedby={fieldErrors.password ? "password-error" : "password-strength"}
                                        className={cn(
                                            "h-16 pl-14 pr-14 bg-slate-50/50 border-transparent rounded-2xl focus:bg-white focus:ring-[6px] focus:ring-slate-100 focus:border-slate-200 transition-all text-lg font-medium shadow-none",
                                            fieldErrors.password && "border-red-200 focus:ring-red-100"
                                        )}
                                        autoComplete="new-password"
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
                                    <p id="password-error" role="alert" className="text-sm font-bold text-red-500 ml-2">{fieldErrors.password}</p>
                                )}
                                <div id="password-strength" className="space-y-1.5 ml-2 mt-1">
                                    {formData.password.length > 0 && (
                                        <div className="flex items-center gap-2 mb-2">
                                            <span className="text-[10px] font-black uppercase tracking-[0.2em] text-slate-400 w-20">
                                                {passwordStrength <= 1 ? "Weak" : passwordStrength <= 3 ? "Fair" : "Strong"}
                                            </span>
                                            <div className="flex-1 h-1.5 rounded-full bg-slate-100 overflow-hidden">
                                                <div
                                                    className={cn(
                                                        "h-full rounded-full transition-all duration-300",
                                                        passwordStrength <= 1 ? "bg-red-400"
                                                        : passwordStrength <= 3 ? "bg-amber-400"
                                                        : "bg-emerald-500"
                                                    )}
                                                    style={{ width: `${(passwordStrength / passwordChecks.length) * 100}%` }}
                                                />
                                            </div>
                                        </div>
                                    )}
                                    <ul className="grid grid-cols-2 gap-x-4 gap-y-1">
                                        {passwordChecks.map((check) => (
                                            <li key={check.id} className={cn("flex items-center gap-1.5 text-[11px] font-bold", check.ok ? "text-emerald-500" : "text-slate-300")}>
                                                {check.ok ? <CheckCircle2 className="h-3.5 w-3.5" /> : <Circle className="h-3.5 w-3.5" />}
                                                {check.label}
                                            </li>
                                        ))}
                                    </ul>
                                </div>
                            </div>
                        </div>

                        <div className="pt-6">
                            <Button
                                type="submit"
                                aria-busy={isLoading || undefined}
                                className={cn(
                                    "w-full h-18 text-white font-black text-xl rounded-[2rem] shadow-2xl transition-all hover:scale-[1.02] active:scale-[0.98] disabled:opacity-70 group",
                                    currentStyle.bg,
                                    "shadow-lg"
                                )}
                                disabled={isLoading}
                            >
                                {isLoading ? (
                                    <span className="flex items-center justify-center gap-3">
                                        <Loader2 className="h-8 w-8 animate-spin" /> Creating Account
                                    </span>
                                ) : (
                                    <span className="flex items-center justify-center gap-3">
                                        Establish Account <ArrowRight className="h-6 w-6 group-hover:translate-x-2 transition-transform duration-300" />
                                    </span>
                                )}
                            </Button>
                        </div>
                    </form>

                    <div className="text-center">
                        <p className="text-slate-400 font-bold">
                            Already part of the network?{" "}
                            <Link href={`/login?role=${formData.role}`} className={cn("font-black hover:underline decoration-4 underline-offset-8 transition-colors", currentStyle.text)}>
                                Sign In
                            </Link>
                        </p>
                    </div>
                </div>
            </div>
        </div>
    )
}

export default function Register() {
    return (
        <Suspense fallback={<div className="min-h-screen flex items-center justify-center bg-white font-black tracking-widest text-slate-300">SYSTEM_INIT...</div>}>
            <RegisterForm />
        </Suspense>
    )
}
