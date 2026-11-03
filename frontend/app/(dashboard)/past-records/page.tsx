"use client";

import { useCallback, useEffect, useState } from "react"
import { useRouter } from "next/navigation"
import { DashboardLayout } from "@/components/layout/DashboardLayout"
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card"
import { PageError, PageSkeleton, PageEmpty } from "@/components/ui/page-states"
import { Heart, FileText, Calendar } from "lucide-react"
import { getApiUrl } from "@/lib/api"

interface HealthRecord {
    recorded_at: string
    blood_pressure_systolic: number
    blood_pressure_diastolic: number
}

interface Prescription {
    prescription_date: string
    doctor_name: string
}

interface PastRecordsData {
    health_records: HealthRecord[]
    prescriptions: Prescription[]
}

export default function PastRecords() {
    const [data, setData] = useState<PastRecordsData | null>(null)
    const [loading, setLoading] = useState(true)
    const [error, setError] = useState<string | null>(null)
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
                router.push('/login')
                return
            }

            try {
                const response = await fetch(getApiUrl("/api/past-records/"), {
                    headers: {
                        'Authorization': `Bearer ${token}`,
                        'Content-Type': 'application/json'
                    },
                    signal: controller.signal
                })

                if (response.status === 401 || response.status === 403) {
                    localStorage.removeItem('token')
                    router.push('/login')
                    return
                }

                if (!response.ok) {
                    throw new Error('Failed to fetch past records')
                }

                const result = await response.json()
                setData(result)
            } catch (err: unknown) {
                if (err instanceof Error) {
                    if (err.name === 'AbortError') return
                    console.error("Past Records Fetch Error:", err)
                    setError(err.message || "Failed to load past records")
                } else {
                    console.error("Past Records Fetch Error:", err)
                    setError("Failed to load past records")
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
                <PageSkeleton stats={0} rows={5} title="Loading Past Records..." />
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

    const healthRecords = data.health_records ?? []
    const prescriptions = data.prescriptions ?? []

    return (
        <DashboardLayout>
            <header className="mb-8">
                <h1 className="text-3xl font-bold tracking-tight text-primary">Past Records</h1>
                <p className="text-muted-foreground mt-1">Your historical health records and prescriptions</p>
            </header>

            {healthRecords.length === 0 && prescriptions.length === 0 ? (
                <PageEmpty
                    icon={Calendar}
                    title="No past records yet"
                    description="Your old health records and prescriptions will show up here."
                    ctaLabel="Add health entry"
                    ctaHref="/health-track/add"
                />
            ) : (
                <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
                    <Card className="border-border shadow-sm h-full">
                        <CardHeader className="bg-transparent border-b border-border/50 pb-4">
                            <CardTitle className="flex items-center gap-2 text-lg">
                                <Heart className="h-5 w-5 text-primary" />
                                Health Records
                            </CardTitle>
                        </CardHeader>
                        <CardContent className="pt-4">
                            <ul className="space-y-3">
                                {healthRecords.slice(0, 5).map((record, i) => (
                                    <li key={i} className="flex justify-between items-center p-3 rounded-lg bg-secondary/30">
                                        <div className="flex items-center gap-2 text-sm font-medium">
                                            <Calendar className="h-4 w-4 text-muted-foreground" />
                                            {record.recorded_at}
                                        </div>
                                        <div className="text-sm text-muted-foreground">
                                            {record.blood_pressure_systolic && (
                                                <span>BP: {record.blood_pressure_systolic}/{record.blood_pressure_diastolic}</span>
                                            )}
                                        </div>
                                    </li>
                                ))}
                                {healthRecords.length === 0 && (
                                    <li className="text-center text-muted-foreground py-4">No records found</li>
                                )}
                            </ul>
                        </CardContent>
                    </Card>

                    <Card className="border-border shadow-sm h-full">
                        <CardHeader className="bg-transparent border-b border-border/50 pb-4">
                            <CardTitle className="flex items-center gap-2 text-lg">
                                <FileText className="h-5 w-5 text-emerald-500" />
                                Prescriptions
                            </CardTitle>
                        </CardHeader>
                        <CardContent className="pt-4">
                            <ul className="space-y-3">
                                {prescriptions.slice(0, 5).map((p, i) => (
                                    <li key={i} className="flex justify-between items-center p-3 rounded-lg bg-secondary/30">
                                        <div className="flex items-center gap-2 text-sm font-medium">
                                            <Calendar className="h-4 w-4 text-muted-foreground" />
                                            {p.prescription_date}
                                        </div>
                                        <div className="text-sm text-muted-foreground">
                                            {p.doctor_name}
                                        </div>
                                    </li>
                                ))}
                                {prescriptions.length === 0 && (
                                    <li className="text-center text-muted-foreground py-4">No prescriptions found</li>
                                )}
                            </ul>
                        </CardContent>
                    </Card>
                </div>
            )}
        </DashboardLayout>
    )
}
