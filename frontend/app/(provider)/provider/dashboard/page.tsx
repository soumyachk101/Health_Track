"use client";

import { useState, useEffect, useCallback } from 'react'
import { Clock, MapPin, DollarSign, CheckCircle, TrendingUp, Package, Home, Settings, ArrowRight, AlertCircle, Inbox, Loader2 } from 'lucide-react'
import Link from "next/link"
import { cn, getStoredToken, clearSession } from "@/lib/utils"
import { Button } from "@/components/ui/button"
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card"
import { DashboardLayout } from "@/components/layout/DashboardLayout"
import { useRouter } from "next/navigation"
import { getApiUrl } from "@/lib/api"
import { useAuth } from "@/components/Providers"

const providerNavItems = [
    { icon: Home, label: "Dashboard", href: "/provider/dashboard" },
    { icon: Package, label: "Services", href: "/provider/services" },
    { icon: MapPin, label: "Requests", href: "/provider/requests" },
    { icon: DollarSign, label: "Earnings", href: "/provider/earnings" },
    { icon: Clock, label: "History", href: "/provider/history" },
    { icon: Settings, label: "Settings", href: "/provider/settings" },
]

interface ServiceRequest {
    id: number
    patient_name?: string
    provider_name?: string
    service_name: string
    price: number | null
    status: 'pending' | 'accepted' | 'completed' | 'declined'
    address: string
    scheduled_date?: string | null
}

export default function ProviderDashboard() {
    const [requests, setRequests] = useState<ServiceRequest[]>([])
    const [isLoading, setIsLoading] = useState(true)
    const [error, setError] = useState<string | null>(null)
    const router = useRouter()
    const { profile } = useAuth()
    // Monogram: initials of the provider's name when available, else "PRO".
    const providerName = `${profile?.user?.first_name ?? ""} ${profile?.user?.last_name ?? ""}`.trim()
    const providerInitials = providerName
        ? providerName.split(/\s+/).map(w => w.charAt(0)).slice(0, 2).join("").toUpperCase()
        : "PRO"
    const [stats, setStats] = useState([
        { label: "Pending Requests", value: "0", icon: Clock, color: "text-amber-500", bg: "bg-amber-50" },
        { label: "Completed Jobs", value: "0", icon: CheckCircle, color: "text-emerald-500", bg: "bg-emerald-50" },
        { label: "Total Earnings", value: "$0", icon: DollarSign, color: "text-blue-500", bg: "bg-blue-50" },
        { label: "Rating", value: "5.0", icon: TrendingUp, color: "text-purple-500", bg: "bg-purple-50" },
    ])
    const [isOnline, setIsOnline] = useState(false)

    const fetchRequests = useCallback(async () => {
        setError(null)
        const token = getStoredToken()
        if (!token) {
            router.push('/login')
            return
        }
        try {
            const response = await fetch(getApiUrl("/api/service-requests"), {
                headers: {
                    'Authorization': `Bearer ${token}`
                }
            })

            if (response.status === 401 || response.status === 403) {
                clearSession()
                router.push('/login')
                return
            }

            if (!response.ok) {
                throw new Error('Failed to load service requests')
            }

            const data = await response.json()
            if (data.success) {
                setRequests(data.requests)
                // Update stats
                const pendingCount = data.requests.filter((r: ServiceRequest) => r.status === 'pending').length
                const completedCount = data.requests.filter((r: ServiceRequest) => r.status === 'completed').length
                const earnings = data.requests
                    .filter((r: ServiceRequest) => r.status === 'completed')
                    .reduce((acc: number, curr: ServiceRequest) => acc + (curr.price ?? 0), 0)

                setStats(prev => prev.map(s => {
                    if (s.label === "Pending Requests") return { ...s, value: pendingCount.toString() }
                    if (s.label === "Completed Jobs") return { ...s, value: completedCount.toString() }
                    if (s.label === "Total Earnings") return { ...s, value: `$${earnings.toFixed(2)}` }
                    return s
                }))
            }
        } catch (err: unknown) {
            if (err instanceof Error) {
                console.error("Failed to fetch requests:", err.message)
                setError("Couldn&apos;t load service requests. Check your connection and try again.")
            } else {
                console.error("Failed to fetch requests:", err)
                setError("Couldn&apos;t load service requests. Check your connection and try again.")
            }
        } finally {
            setIsLoading(false)
        }
    }, [router])

    useEffect(() => {
        fetchRequests()
    }, [fetchRequests])

    const handleAction = async (id: number, action: 'accept' | 'decline' | 'complete') => {
        try {
            const token = getStoredToken()

            const response = await fetch(getApiUrl(`/api/service-requests/${id}/action`), {
                method: 'POST',
                headers: {
                    'Content-Type': 'application/json',
                    'Authorization': `Bearer ${token}`,
                },
                body: JSON.stringify({ action })
            })

            const data = await response.json()
            if (data.success) {
                fetchRequests() // Refresh list
            }
        } catch (err: unknown) {
            if (err instanceof Error) {
                console.error("Action failed:", err.message)
            } else {
                console.error("Action failed:", err)
            }
        }
    }

    return (
        <DashboardLayout sidebarItems={providerNavItems}>
            <div className="flex flex-col min-w-0 overflow-hidden gap-4 h-full">
                {/* Header */}
                <header className="bg-white shadow-clay-card rounded-3xl p-6 flex flex-col md:flex-row items-center justify-between gap-4 mb-4">
                    <div className="flex items-center gap-4 w-full md:w-auto">
                        <h1 className="text-xl font-bold text-slate-800">Provider Dashboard</h1>
                        <span className={cn("px-2 py-1 rounded-full text-xs font-semibold",
                            isOnline ? "bg-green-100 text-green-700" : "bg-slate-100 text-slate-600"
                        )}>
                            {isOnline ? "Online" : "Offline"}
                        </span>
                    </div>

                    <div className="flex items-center gap-4 w-full md:w-auto justify-end">
                        <div className="flex items-center gap-2 bg-slate-50 p-1 rounded-xl border border-slate-200">
                            <Button
                                variant={isOnline ? "default" : "ghost"}
                                size="sm"
                                onClick={() => setIsOnline(true)}
                                className={cn("rounded-lg text-xs", isOnline && "bg-green-600 hover:bg-green-700")}
                            >
                                Go Online
                            </Button>
                            <Button
                                variant={!isOnline ? "default" : "ghost"}
                                size="sm"
                                onClick={() => setIsOnline(false)}
                                className={cn("rounded-lg text-xs", !isOnline && "bg-slate-600 hover:bg-slate-700")}
                            >
                                Go Offline
                            </Button>
                        </div>
                        <div className="h-8 w-8 rounded-full bg-gradient-to-br from-indigo-500 to-indigo-700 flex items-center justify-center text-white text-[11px] font-bold border border-white shadow-sm" aria-label="Provider avatar">
                            {providerInitials}
                        </div>
                    </div>
                </header>

                {/* Dashboard Content */}
                <div className="flex-1 overflow-y-auto">

                    {/* Stats Grid */}
                    <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-4 mb-8">
                        {stats.map((stat, idx) => (
                            <Card key={idx} className="hover:-translate-y-1 transition-transform border-border shadow-sm">
                                <CardContent className="p-6 flex items-center justify-between">
                                    <div>
                                        <p className="text-sm font-medium text-slate-500 mb-1">{stat.label}</p>
                                        <h3 className="text-2xl font-bold text-slate-900">{stat.value}</h3>
                                    </div>
                                    <div className={cn("h-12 w-12 rounded-xl flex items-center justify-center", stat.bg, stat.color)}>
                                        <stat.icon className="h-6 w-6" />
                                    </div>
                                </CardContent>
                            </Card>
                        ))}
                    </div>

                    <div className="grid lg:grid-cols-3 gap-8">
                        {/* Service Requests */}
                        <div className="lg:col-span-2 space-y-6">
                            <h2 className="text-lg font-bold text-slate-800">Incoming Requests</h2>
                            <div className="space-y-4">
                                {isLoading ? (
                                    <div className="text-center text-slate-500 py-8 flex items-center justify-center gap-2">
                                        <Loader2 className="h-4 w-4 animate-spin" />
                                        <span>Loading requests...</span>
                                    </div>
                                ) : error ? (
                                    <div>
                                        <div className="p-5 rounded-3xl bg-red-50 text-red-600 border border-red-100 flex items-start gap-4">
                                            <AlertCircle className="h-6 w-6 mt-0.5 shrink-0" />
                                            <div className="flex-1">
                                                <p className="text-base font-bold leading-tight">Couldn&apos;t load service requests</p>
                                                <p className="text-sm text-red-500 mt-1">{error}</p>
                                            </div>
                                        </div>
                                        <div className="flex justify-center mt-6">
                                            <Button onClick={() => { setError(null); setIsLoading(true); fetchRequests() }} className="h-10">
                                                Try again
                                            </Button>
                                        </div>
                                    </div>
                                ) : requests.length === 0 ? (
                                    <div className="bg-white rounded-3xl shadow-clay-card border border-border/50 p-12 text-center space-y-4">
                                        <div className="h-14 w-14 rounded-2xl bg-blue-50 text-blue-500 flex items-center justify-center mx-auto">
                                            <Inbox className="h-7 w-7" />
                                        </div>
                                        <div className="space-y-1">
                                            <h3 className="text-base font-semibold text-slate-800">No incoming service requests</h3>
                                            <p className="text-sm text-slate-500 max-w-sm mx-auto">New service requests from patients will appear here.</p>
                                        </div>
                                    </div>
                                ) : (
                                    requests.map((req) => (
                                        <Card key={req.id} className="border-border shadow-sm hover:shadow-md transition-shadow">
                                            <CardContent className="p-0">
                                                <div className="p-6 flex flex-col sm:flex-row gap-6">
                                                    {/* Date Box */}
                                                    <div className="flex-shrink-0">
                                                        <div className="h-16 w-16 bg-blue-50 rounded-2xl flex flex-col items-center justify-center text-blue-700 border border-blue-100">
                                                            <span className="text-xs font-bold uppercase">{req.scheduled_date ? new Date(req.scheduled_date).toLocaleString('default', { month: 'short' }) : '—'}</span>
                                                            <span className="text-2xl font-bold">{req.scheduled_date ? new Date(req.scheduled_date).getDate() : '—'}</span>
                                                        </div>
                                                    </div>

                                                    {/* Content */}
                                                    <div className="flex-1">
                                                        <div className="flex justify-between items-start mb-2">
                                                            <div>
                                                                <h3 className="text-lg font-bold text-slate-900">{req.service_name}</h3>
                                                                <p className="text-slate-500 text-sm flex items-center gap-1 mt-1">
                                                                    <MapPin className="h-3 w-3" />
                                                                    {req.address}
                                                                </p>
                                                            </div>
                                                            <div className="text-right">
                                                                <span className="block text-lg font-bold text-slate-900">${req.price ?? 0}</span>
                                                                <span className={cn("text-xs font-medium px-2 py-0.5 rounded-full",
                                                                    req.status === 'pending' ? "bg-amber-100 text-amber-700" :
                                                                        req.status === 'accepted' ? "bg-blue-100 text-blue-700" :
                                                                            req.status === 'completed' ? "bg-green-100 text-green-700" : "bg-slate-100 text-slate-600"
                                                                )}>
                                                                    {req.status}
                                                                </span>
                                                            </div>
                                                        </div>

                                                        {/* Actions */}
                                                        {req.status === 'pending' && (
                                                            <div className="flex gap-3 mt-4 pt-4 border-t border-slate-100">
                                                                <Button onClick={() => handleAction(req.id, 'accept')} className="flex-1 bg-slate-900 hover:bg-slate-800 text-white h-9">
                                                                    Accept Request
                                                                </Button>
                                                                <Button onClick={() => handleAction(req.id, 'decline')} variant="outline" className="flex-1 border-slate-200 hover:bg-slate-50 h-9 text-slate-700">
                                                                    Decline
                                                                </Button>
                                                            </div>
                                                        )}
                                                        {req.status === 'accepted' && (
                                                            <div className="flex gap-3 mt-4 pt-4 border-t border-slate-100">
                                                                <Button onClick={() => handleAction(req.id, 'complete')} className="flex-1 bg-green-600 hover:bg-green-700 text-white h-9">
                                                                    Mark Completed
                                                                </Button>
                                                            </div>
                                                        )}
                                                    </div>
                                                </div>
                                            </CardContent>
                                        </Card>
                                    ))
                                )}
                            </div>
                        </div>

                        {/* Sidebar */}
                        <div className="space-y-6">
                            <Card className="bg-slate-900 text-white border-none shadow-xl">
                                <CardHeader>
                                    <CardTitle>Availability</CardTitle>
                                </CardHeader>
                                <CardContent>
                                    <div className="space-y-4">
                                        <div className="flex justify-between items-center">
                                            <span className="text-slate-400">Status</span>
                                            <span className={cn("font-medium", isOnline ? "text-emerald-400" : "text-slate-300")}>
                                                {isOnline ? "Online" : "Offline"}
                                            </span>
                                        </div>
                                        <div className="flex justify-between items-center">
                                            <span className="text-slate-400">Pending Requests</span>
                                            <span className="font-medium">{requests.filter((r) => r.status === 'pending').length}</span>
                                        </div>
                                        <div className="flex justify-between items-center">
                                            <span className="text-slate-400">Completed Jobs</span>
                                            <span className="font-medium">{requests.filter((r) => r.status === 'completed').length}</span>
                                        </div>
                                        <div className="pt-4 mt-4 border-t border-slate-800">
                                            <Button
                                                asChild
                                                className="w-full bg-white text-slate-900 hover:bg-slate-100 font-semibold"
                                            >
                                                <Link href="/provider/requests">
                                                    View All Requests <ArrowRight className="ml-1 h-4 w-4" />
                                                </Link>
                                            </Button>
                                        </div>
                                    </div>
                                </CardContent>
                            </Card>
                        </div>
                    </div>

                </div>
            </div>
        </DashboardLayout>
    )
}
