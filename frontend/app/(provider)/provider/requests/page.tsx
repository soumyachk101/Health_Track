"use client";

import { useState, useEffect, useCallback } from 'react'
import { Clock, MapPin, DollarSign, Package, Home, Settings, AlertCircle, Inbox, Loader2 } from 'lucide-react'
import { cn, getStoredToken, clearSession } from "@/lib/utils"
import { Button } from "@/components/ui/button"
import { Card, CardContent } from "@/components/ui/card"
import { DashboardLayout } from "@/components/layout/DashboardLayout"
import { useRouter } from "next/navigation"
import { getApiUrl } from "@/lib/api"

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
    service_name: string
    address: string
    price: number | null
    status: 'pending' | 'accepted' | 'completed' | 'declined'
    scheduled_date?: string | null
    patient_name?: string
}

export default function ProviderRequests() {
    const [requests, setRequests] = useState<ServiceRequest[]>([])
    const [isLoading, setIsLoading] = useState(true)
    const [error, setError] = useState<string | null>(null)
    const router = useRouter()

    const fetchRequests = useCallback(async () => {
        const token = getStoredToken()
        if (!token) {
            router.push('/login')
            return
        }
        try {
            setError(null)
            const response = await fetch(getApiUrl("/api/service-requests"), {
                headers: { 'Authorization': `Bearer ${token}` }
            })

            if (response.status === 401 || response.status === 403) {
                clearSession()
                router.push('/login')
                return
            }

            const data = await response.json()
            if (data.success) {
                setRequests(data.requests)
            } else {
                throw new Error(data.message || 'Failed to load service requests')
            }
        } catch (err: unknown) {
            if (err instanceof Error) {
                console.error("Failed to fetch requests:", err.message)
                setError(err.message)
            } else {
                console.error("Failed to fetch requests:", err)
                setError("Failed to load service requests")
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
                fetchRequests() 
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
            <div className="flex flex-col gap-6 h-full">
                <header className="h-20 bg-white shadow-clay-card rounded-3xl flex items-center justify-between px-8">
                    <h1 className="text-xl font-bold text-slate-800">Service Requests</h1>
                </header>

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
                            <Card key={req.id} className="border-border shadow-sm">
                                <CardContent className="p-6 flex flex-col sm:flex-row gap-6 items-center justify-between">
                                    <div className="flex items-center gap-6 flex-1">
                                        <div className="h-16 w-16 bg-blue-50 rounded-2xl flex flex-col items-center justify-center text-blue-700 border border-blue-100 flex-shrink-0">
                                            <span className="text-xs font-bold uppercase">
                                                {req.scheduled_date ? new Date(req.scheduled_date).toLocaleString('default', { month: 'short' }) : '—'}
                                            </span>
                                            <span className="text-2xl font-bold">
                                                {req.scheduled_date ? new Date(req.scheduled_date).getDate() : '—'}
                                            </span>
                                        </div>
                                        <div>
                                            <h3 className="text-lg font-bold text-slate-900">{req.service_name}</h3>
                                            <p className="text-slate-500 text-sm flex items-center gap-1 mt-1">
                                                <MapPin className="h-3 w-3" /> {req.address}
                                            </p>
                                            <p className="text-xs font-semibold text-slate-400 mt-1">Patient: {req.patient_name}</p>
                                        </div>
                                    </div>
                                    
                                    <div className="flex items-center gap-6">
                                        <div className="text-right">
                                            <span className="block text-xl font-bold text-slate-900">${req.price ?? 0}</span>
                                            <span className={cn("text-xs font-semibold px-3 py-1 rounded-full inline-block mt-1",
                                                req.status === 'pending' ? "bg-amber-100 text-amber-700" :
                                                req.status === 'accepted' ? "bg-blue-100 text-blue-700" :
                                                req.status === 'completed' ? "bg-green-100 text-green-700" : "bg-slate-100 text-slate-600"
                                            )}>
                                                {req.status}
                                            </span>
                                        </div>

                                        <div className="flex gap-2">
                                            {req.status === 'pending' && (
                                                <>
                                                    <Button onClick={() => handleAction(req.id, 'accept')} className="bg-slate-900 hover:bg-slate-800 text-white">
                                                        Accept
                                                    </Button>
                                                    <Button onClick={() => handleAction(req.id, 'decline')} variant="outline" className="border-slate-200 hover:bg-slate-50 text-slate-700">
                                                        Decline
                                                    </Button>
                                                </>
                                            )}
                                            {req.status === 'accepted' && (
                                                <Button onClick={() => handleAction(req.id, 'complete')} className="bg-green-600 hover:bg-green-700 text-white">
                                                    Complete
                                                </Button>
                                            )}
                                        </div>
                                    </div>
                                </CardContent>
                            </Card>
                        ))
                    )}
                </div>
            </div>
        </DashboardLayout>
    )
}
