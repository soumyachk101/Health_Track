"use client";

import { useCallback, useEffect, useState } from "react"
import { DashboardLayout } from "@/components/layout/DashboardLayout"
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card"
import { PageError, PageEmpty, PageSkeleton } from "@/components/ui/page-states"
import { Heart, Droplet, Weight, Activity, Pill, Moon, Plus, FileText, Brain, Footprints } from "lucide-react"
import { useRouter } from "next/navigation"
import Link from "next/link"
import { getApiUrl } from "@/lib/api"

// Define interfaces for data passed from API
interface DashboardData {
    user: {
        name: string
        email: string
    }
    latest_record: {
        blood_pressure_systolic: number | null
        blood_pressure_diastolic: number | null
        bp_status: string
        blood_sugar: string | null
        weight: string | null
        heart_rate: number | null
        recorded_at: string
    } | null
    active_medicines: number
    active_medicines_count: number
    latest_mental_health: {
        sleep_hours: string | null
        mood_score: number
        stress_level: number
    } | null
    recent_activities: Array<{
        action: string
        action_display: string
        details: string
        created_at: string
        created_at_since: string
    }>
}

export default function Dashboard() {
    const [data, setData] = useState<DashboardData | null>(null)
    const [loading, setLoading] = useState(true)
    const [error, setError] = useState<string | null>(null)
    // Bumping this key re-runs the fetch (Retry action).
    const [reloadKey, setReloadKey] = useState(0)
    const router = useRouter()

    const refetch = useCallback(() => {
        setError(null)
        setData(null)
        setReloadKey(k => k + 1)
    }, [])

    useEffect(() => {
        const controller = new AbortController()
        const fetchData = async () => {
            setLoading(true)
            setError(null)

            const token = localStorage.getItem('token')
            if (!token) {
                // No session at all: skip the fetch and go to login.
                router.push('/login')
                return
            }

            try {
                const response = await fetch(getApiUrl("/api/dashboard/"), {
                    headers: {
                        'Authorization': `Bearer ${token}`,
                        'Content-Type': 'application/json'
                    },
                    signal: controller.signal
                })

                // A 401/403 means the session is dead or wrong role — redirect
                // after the fetch attempt, not before.
                if (response.status === 401 || response.status === 403) {
                    localStorage.removeItem('token')
                    router.push('/login')
                    return
                }

                if (!response.ok) {
                    throw new Error('Failed to fetch dashboard data')
                }

                const result = await response.json()
                setData(result)
            } catch (err: unknown) {
                if (err instanceof Error) {
                    if (err.name === 'AbortError') return
                    console.error("Dashboard Fetch Error:", err)
                    setError(err.message || "Failed to load dashboard")
                } else {
                    console.error("Dashboard Fetch Error:", err)
                    setError("Failed to load dashboard")
                }
            } finally {
                if (!controller.signal.aborted) {
                    setLoading(false)
                }
            }
        }

        fetchData()
        return () => controller.abort()
    }, [router, reloadKey])

    if (loading) {
        return (
            <DashboardLayout>
                <PageSkeleton stats={4} rows={2} title="Loading dashboard" />
            </DashboardLayout>
        )
    }

    if (error) {
        return (
            <DashboardLayout>
                <PageError message={error} onRetry={refetch} />
            </DashboardLayout>
        )
    }

    if (!data) return null

    // Helper to safely get display value
    const getVal = (val: string | number | undefined | null, suffix: string = "") => {
        if (val === undefined || val === null || val === "" || val === "None") return `-- ${suffix}`.trim()
        return `${val} ${suffix}`.trim()
    }

    const hasVal = (val: string | number | undefined | null) => {
        return val !== undefined && val !== null && val !== "" && val !== "None"
    }

    // Stats Array
    const stats = [
        {
            label: "Blood Pressure",
            value: hasVal(data.latest_record?.blood_pressure_systolic)
                ? `${data.latest_record?.blood_pressure_systolic}/${data.latest_record?.blood_pressure_diastolic} mmHg`
                : "--/-- mmHg",
            status: data.latest_record?.bp_status || "No data",
            icon: Heart,
            color: "text-rose-500",
            bg: "bg-rose-500/10"
        },
        {
            label: "Blood Sugar",
            value: getVal(data.latest_record?.blood_sugar, "mg/dL"),
            status: hasVal(data.latest_record?.blood_sugar) ? "Recorded" : "No data",
            icon: Droplet,
            color: "text-sky-500",
            bg: "bg-sky-500/10"
        },
        {
            label: "Weight",
            value: getVal(data.latest_record?.weight, "Kg"),
            status: hasVal(data.latest_record?.weight) ? "Recorded" : "No data",
            icon: Weight,
            color: "text-emerald-500",
            bg: "bg-emerald-500/10"
        },
        {
            label: "Heart Rate",
            value: getVal(data.latest_record?.heart_rate, "bpm"),
            status: hasVal(data.latest_record?.heart_rate) ? "Recorded" : "No data",
            icon: Activity,
            color: "text-amber-500",
            bg: "bg-amber-500/10"
        }
    ]

    const activeMeds = data.active_medicines || 0
    const sleepHours = data.latest_mental_health?.sleep_hours

    return (
        <DashboardLayout>
            <div className="pb-24 md:pb-0">
                <header className="mb-8 flex items-center justify-between">
                    <div>
                        <h1 className="text-3xl font-bold tracking-tight">Dashboard</h1>
                        <p className="text-muted-foreground mt-1">Welcome back, {data.user.name}</p>
                    </div>
                </header>

                {/* Bento Grid Layout */}
                <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-6 mb-8">
                    {stats.map((stat) => (
                        <Card key={stat.label} className="border-border/50 bg-card/50 backdrop-blur">
                            <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
                                <CardTitle className="text-sm font-medium text-muted-foreground">{stat.label}</CardTitle>
                                <div className={`p-2 rounded-full ${stat.bg} ${stat.color}`}>
                                    <stat.icon className="h-4 w-4" />
                                </div>
                            </CardHeader>
                            <CardContent>
                                <div className="text-2xl font-bold animate-in fade-in slide-in-from-bottom-2">{stat.value}</div>
                                <p className="text-xs text-muted-foreground mt-1">{stat.status}</p>
                            </CardContent>
                        </Card>
                    ))}
                </div>

                <div className="grid grid-cols-1 lg:grid-cols-3 gap-8">
                    {/* Wellness Overview */}
                    <div className="lg:col-span-2 grid grid-cols-1 md:grid-cols-2 gap-6">
                        <Card className="border-border/50 bg-card/50">
                            <CardHeader className="flex flex-row items-center space-y-0 pb-2 gap-4">
                                <div className="p-2 rounded-full bg-violet-500/10 text-violet-500">
                                    <Pill className="h-5 w-5" />
                                </div>
                                <div>
                                    <CardTitle className="text-base">Active Medicines</CardTitle>
                                    <p className="text-xs text-muted-foreground">Currently taking</p>
                                </div>
                            </CardHeader>
                            <CardContent>
                                <div className="text-3xl font-bold">{activeMeds}</div>
                            </CardContent>
                        </Card>
                        <Card className="border-border/50 bg-card/50">
                            <CardHeader className="flex flex-row items-center space-y-0 pb-2 gap-4">
                                <div className="p-2 rounded-full bg-indigo-500/10 text-indigo-500">
                                    <Moon className="h-5 w-5" />
                                </div>
                                <div>
                                    <CardTitle className="text-base">Sleep Hours</CardTitle>
                                    <p className="text-xs text-muted-foreground">Last night</p>
                                </div>
                            </CardHeader>
                            <CardContent>
                                <div className="text-3xl font-bold">
                                    {hasVal(sleepHours) ? sleepHours : "--"} <span className="text-base font-normal text-muted-foreground">hrs</span>
                                </div>
                            </CardContent>
                        </Card>

                        {/* Recent Activity */}
                        <Card className="col-span-1 md:col-span-2 border-border/50 bg-card/50">
                            <CardHeader>
                                <CardTitle>Recent Activity</CardTitle>
                            </CardHeader>
                            <CardContent>
                                <div className="space-y-4">
                                    {data.recent_activities && data.recent_activities.length > 0 ? (
                                        data.recent_activities.map((activity, i) => (
                                            <div key={i} className="flex items-center gap-4 p-3 rounded-lg hover:bg-muted/50 transition-colors">
                                                <div className="h-2 w-2 rounded-full bg-primary" />
                                                <div className="flex-1">
                                                    <p className="text-sm font-medium">{activity.action_display}</p>
                                                    <p className="text-xs text-muted-foreground">{activity.details}</p>
                                                </div>
                                                <span className="text-xs text-muted-foreground">{activity.created_at_since} ago</span>
                                            </div>
                                        ))
                                    ) : (
                                        <PageEmpty
                                            icon={Activity}
                                            title="No recent activity"
                                            description="Your recent health actions will show up here."
                                            ctaLabel="Add health entry"
                                            ctaHref="/health-track/add"
                                        />
                                    )}
                                </div>
                            </CardContent>
                        </Card>
                    </div>

                    {/* Quick Actions */}
                    <div className="space-y-4">
                        <h3 className="text-lg font-semibold">Quick Actions</h3>
                        {[
                            { label: "Add Health Record", href: "/health-records/add", icon: Plus },
                            { label: "Add Medicine", href: "/medicines/add", icon: Pill },
                            { label: "Add Prescription", href: "/prescriptions/add", icon: FileText },
                            { label: "Mental Health Tools", href: "/mental-health", icon: Brain },
                            { label: "Lifestyle Tracking", href: "/lifestyle", icon: Footprints },
                        ].map((action) => (
                            <Link
                                key={action.label}
                                href={action.href}
                                className="flex items-center gap-4 p-4 rounded-xl border border-border bg-card hover:bg-accent transition-all group shadow-sm hover:shadow-md focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2"
                            >
                                <div className="h-10 w-10 rounded-full bg-primary/10 flex items-center justify-center text-primary group-hover:bg-primary group-hover:text-primary-foreground transition-colors">
                                    <action.icon className="h-5 w-5" />
                                </div>
                                <span className="font-medium">{action.label}</span>
                            </Link>
                        ))}
                    </div>
                </div>
            </div>

            {/* Sticky mobile bottom CTA — mobile only */}
            <div className="md:hidden fixed bottom-0 left-0 right-0 z-30 bg-white/95 backdrop-blur border-t border-border shadow-[0_-4px_16px_rgba(0,0,0,0.06)] pb-[env(safe-area-inset-bottom)]">
                <div className="p-4">
                    <Link
                        href="/health-track/add"
                        className="flex w-full items-center justify-center gap-2 h-12 rounded-2xl bg-primary text-primary-foreground font-semibold shadow-skeuo-sm hover:scale-[1.01] active:scale-95 transition-all focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2"
                    >
                        <Plus className="h-5 w-5" />
                        Add health entry
                    </Link>
                </div>
            </div>
        </DashboardLayout>
    )
}
