"use client";

import { useCallback, useEffect, useState } from "react"
import { DashboardLayout } from "@/components/layout/DashboardLayout"
import { Card } from "@/components/ui/card"
import { PageError, PageSkeleton, PageEmpty } from "@/components/ui/page-states"
import { Button } from "@/components/ui/button"
import { Plus, User, Hospital, Calendar, FileText } from "lucide-react"
import Link from "next/link"
import { useRouter } from "next/navigation"
import { getApiUrl } from "@/lib/api"

interface Prescription {
    prescription_date: string
    doctor_name: string
    hospital_name: string
    diagnosis: string
    follow_up_date: string
}

interface PrescriptionsData {
    prescriptions: Prescription[]
}

export default function Prescriptions() {
    const [data, setData] = useState<PrescriptionsData | null>(null)
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
                const response = await fetch(getApiUrl("/api/prescriptions/"), {
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
                    throw new Error('Failed to fetch prescriptions')
                }

                const result = await response.json()
                setData(result)
            } catch (err: unknown) {
                if (err instanceof Error) {
                    if (err.name === 'AbortError') return
                    console.error("Prescriptions Fetch Error:", err)
                    setError(err.message || "Failed to load prescriptions")
                } else {
                    console.error("Prescriptions Fetch Error:", err)
                    setError("Failed to load prescriptions")
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
                <PageSkeleton stats={0} rows={4} title="Loading Prescriptions..." />
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

    const prescriptions = data?.prescriptions ?? []

    return (
        <DashboardLayout>
            <header className="mb-8 flex items-center justify-between">
                <div>
                    <h1 className="text-3xl font-bold tracking-tight text-primary">Prescriptions</h1>
                    <p className="text-muted-foreground mt-1">Manage your digital prescriptions</p>
                </div>
                <Link href="/prescriptions/add">
                    <Button className="gap-2 bg-primary hover:bg-primary/90 text-primary-foreground">
                        <Plus className="h-4 w-4" />
                        Add Prescription
                    </Button>
                </Link>
            </header>

            {prescriptions.length === 0 ? (
                <PageEmpty
                    icon={FileText}
                    title="No prescriptions yet"
                    description="Save your first digital prescription to keep everything in one place."
                    ctaLabel="Add prescription"
                    ctaHref="/prescriptions/add"
                />
            ) : (
                <Card className="border-border shadow-sm overflow-hidden">
                    <div className="overflow-x-auto">
                        <table className="w-full min-w-[640px] text-sm text-left">
                            <thead className="text-xs text-muted-foreground bg-secondary/50 border-b border-border">
                                <tr>
                                    <th scope="col" className="px-6 py-4 font-medium">Date</th>
                                    <th scope="col" className="px-6 py-4 font-medium">Doctor</th>
                                    <th scope="col" className="px-6 py-4 font-medium">Hospital</th>
                                    <th scope="col" className="px-6 py-4 font-medium">Diagnosis</th>
                                    <th scope="col" className="px-6 py-4 font-medium">Follow-up</th>
                                </tr>
                            </thead>
                            <tbody className="divide-y divide-border">
                                {prescriptions.map((p, i) => (
                                    <tr key={i} className="bg-card hover:bg-muted/50 transition-colors">
                                        <td className="px-6 py-4">
                                            <div className="flex items-center gap-2 text-foreground font-medium">
                                                <Calendar className="h-3 w-3 text-muted-foreground" />
                                                {p.prescription_date}
                                            </div>
                                        </td>
                                        <td className="px-6 py-4">
                                            <div className="flex items-center gap-2">
                                                <User className="h-3 w-3 text-primary" />
                                                {p.doctor_name}
                                            </div>
                                        </td>
                                        <td className="px-6 py-4">
                                            <div className="flex items-center gap-2">
                                                <Hospital className="h-3 w-3 text-muted-foreground" />
                                                {p.hospital_name || "-"}
                                            </div>
                                        </td>
                                        <td className="px-6 py-4 truncate max-w-[200px]" title={p.diagnosis}>
                                            {p.diagnosis}
                                        </td>
                                        <td className="px-6 py-4 text-muted-foreground">
                                            {p.follow_up_date || "-"}
                                        </td>
                                    </tr>
                                ))}
                            </tbody>
                        </table>
                    </div>
                </Card>
            )}
        </DashboardLayout>
    )
}
