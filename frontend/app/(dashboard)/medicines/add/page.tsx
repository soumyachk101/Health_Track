"use client";

import { useState } from "react"
import { useRouter } from "next/navigation"
import { DashboardLayout } from "@/components/layout/DashboardLayout"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Card, CardContent, CardHeader, CardTitle, CardDescription } from "@/components/ui/card"
import { ErrorBanner, FieldError } from "@/components/ui/page-states"
import { Loader2, Pill, CheckCircle } from "lucide-react"
import { getApiUrl } from "@/lib/api"

type FieldErrors = Record<string, string>

function validateMedicine(values: Record<string, string>): FieldErrors {
    const errors: FieldErrors = {}

    if (!(values.name ?? "").trim()) {
        errors.name = "Medicine name is required."
    }
    if (!(values.dosage ?? "").trim()) {
        errors.dosage = "Dosage is required."
    }

    const start = (values.start_date ?? "").trim()
    const end = (values.end_date ?? "").trim()
    if (end && start && end <= start) {
        errors.end_date = "End date must be after the start date."
    }

    return errors
}

export default function AddMedicine() {
    const [isLoading, setIsLoading] = useState(false)
    const [fieldErrors, setFieldErrors] = useState<FieldErrors>({})
    const [formError, setFormError] = useState<string | null>(null)
    const [success, setSuccess] = useState(false)
    const router = useRouter()

    const describedBy = (field: string, extra: string[] = []) => {
        const ids = extra.filter(Boolean)
        if (fieldErrors[field]) ids.push(`${field}-error`)
        return ids.join(" ") || undefined
    }

    const inputInvalid = (field: string) =>
        fieldErrors[field] ? "border-red-300 shadow-skeuo-inset-md bg-red-50" : undefined

    const selectInvalid = (field: string) =>
        fieldErrors[field]
            ? "flex h-10 w-full rounded-md border border-red-300 bg-red-50 px-3 py-2 text-sm ring-offset-background file:border-0 file:bg-transparent file:text-sm file:font-medium placeholder:text-muted-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 disabled:cursor-not-allowed disabled:opacity-50"
            : "flex h-10 w-full rounded-md border border-input bg-background px-3 py-2 text-sm ring-offset-background file:border-0 file:bg-transparent file:text-sm file:font-medium placeholder:text-muted-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 disabled:cursor-not-allowed disabled:opacity-50"

    const handleSubmit = async (e: React.FormEvent<HTMLFormElement>) => {
        e.preventDefault()
        if (isLoading || success) return // prevent duplicate submits

        const token = localStorage.getItem('token')
        if (!token) {
            router.push('/login')
            return
        }

        const formData = new FormData(e.currentTarget)
        const values: Record<string, string> = {}
        for (const [k, v] of Object.entries(Object.fromEntries(formData.entries()))) {
            values[k] = (v ?? "").toString()
        }

        const errors = validateMedicine(values)
        setFieldErrors(errors)
        setFormError(null)
        if (Object.keys(errors).length > 0) {
            setFormError("Please fix the highlighted fields before saving.")
            return
        }

        setIsLoading(true)

        // The Go backend binds JSON fields only; send the text fields as JSON (no CSRF).
        const body = {
            name: (values.name ?? "").trim(),
            dosage: (values.dosage ?? "").trim(),
            frequency: (values.frequency ?? "once").toString(),
            start_date: (values.start_date ?? "").trim(),
            end_date: (values.end_date ?? "").toString() || null,
            prescribed_by: (values.prescribed_by ?? "").toString(),
            notes: (values.notes ?? "").toString()
        }

        try {
            const response = await fetch(getApiUrl("/api/medicines/add"), {
                method: 'POST',
                headers: {
                    'Content-Type': 'application/json',
                    'Authorization': `Bearer ${token}`
                },
                body: JSON.stringify(body)
            })

            if (response.status === 401 || response.status === 403) {
                localStorage.removeItem('token')
                router.push('/login')
                return
            }

            if (response.ok) {
                setSuccess(true)
                // Show the real success state briefly, then navigate away.
                setTimeout(() => router.push('/medicines'), 800)
            } else {
                setFormError("Failed to save the medicine. Please try again.")
            }
        } catch (error: unknown) {
            if (error instanceof Error) {
                console.error("Error adding medicine:", error.message)
            } else {
                console.error("Error adding medicine:", error)
            }
            setFormError("Something went wrong while saving the medicine. Please try again.")
        } finally {
            setIsLoading(false)
        }
    }

    return (
        <DashboardLayout>
            <header className="mb-8">
                <h1 className="text-3xl font-bold tracking-tight text-primary">Add Medicine</h1>
                <p className="text-muted-foreground mt-1">Schedule a new medication</p>
            </header>

            <div className="max-w-2xl mx-auto">
                <Card className="border-border/50 bg-card/50 backdrop-blur">
                    <CardHeader>
                        <CardTitle>Medicine Details</CardTitle>
                        <CardDescription>Enter dosage and schedule information</CardDescription>
                    </CardHeader>
                    <CardContent>
                        <form method="POST" onSubmit={handleSubmit} noValidate className="space-y-6">

                            {success ? (
                                <div className="p-4 rounded-3xl bg-emerald-50 text-emerald-700 border border-emerald-100 flex items-start gap-3 text-sm">
                                    <CheckCircle className="h-5 w-5 shrink-0" />
                                    <div>
                                        <p className="font-semibold">Medicine saved</p>
                                        <p className="text-emerald-600">Taking you back to your medicines…</p>
                                    </div>
                                </div>
                            ) : (
                                <>
                                    {formError && <ErrorBanner>{formError}</ErrorBanner>}

                                    <div className="grid md:grid-cols-2 gap-4">
                                        <div className="space-y-2">
                                            <Label htmlFor="name">Medicine Name</Label>
                                            <div className="relative">
                                                <Pill className="absolute left-3 top-2.5 h-4 w-4 text-muted-foreground" />
                                                <Input
                                                    id="name"
                                                    name="name"
                                                    placeholder="e.g. Paracetamol"
                                                    className={`pl-10 ${inputInvalid("name") || ""}`}
                                                    aria-invalid={fieldErrors.name ? true : undefined}
                                                    aria-describedby={describedBy("name")}
                                                />
                                            </div>
                                            <FieldError id="name-error" message={fieldErrors.name} />
                                        </div>
                                        <div className="space-y-2">
                                            <Label htmlFor="dosage">Dosage</Label>
                                            <Input
                                                id="dosage"
                                                name="dosage"
                                                placeholder="e.g. 500mg"
                                                className={inputInvalid("dosage") || "w-full"}
                                                aria-invalid={fieldErrors.dosage ? true : undefined}
                                                aria-describedby={describedBy("dosage")}
                                            />
                                            <FieldError id="dosage-error" message={fieldErrors.dosage} />
                                        </div>
                                    </div>

                                    <div className="grid md:grid-cols-2 gap-4">
                                        <div className="space-y-2">
                                            <Label htmlFor="frequency">Frequency</Label>
                                            <select
                                                id="frequency"
                                                name="frequency"
                                                className={selectInvalid("frequency")}
                                            >
                                                <option value="once">Once Daily</option>
                                                <option value="twice">Twice Daily</option>
                                                <option value="thrice">Three Times Daily</option>
                                                <option value="asneeded">As Needed</option>
                                            </select>
                                        </div>
                                        <div className="space-y-2">
                                            <Label htmlFor="prescribed_by">Prescribed By</Label>
                                            <Input id="prescribed_by" name="prescribed_by" placeholder="Doctor's name" />
                                        </div>
                                    </div>

                                    <div className="grid md:grid-cols-2 gap-4">
                                        <div className="space-y-2">
                                            <Label htmlFor="start_date">Start Date</Label>
                                            <Input
                                                id="start_date"
                                                name="start_date"
                                                type="date"
                                                className={inputInvalid("start_date") || "w-full"}
                                                aria-invalid={fieldErrors.start_date ? true : undefined}
                                                aria-describedby={describedBy("start_date")}
                                            />
                                            <FieldError id="start_date-error" message={fieldErrors.start_date} />
                                        </div>
                                        <div className="space-y-2">
                                            <Label htmlFor="end_date">End Date (Optional)</Label>
                                            <Input
                                                id="end_date"
                                                name="end_date"
                                                type="date"
                                                className={inputInvalid("end_date") || "w-full"}
                                                aria-invalid={fieldErrors.end_date ? true : undefined}
                                                aria-describedby={describedBy("end_date")}
                                            />
                                            <FieldError id="end_date-error" message={fieldErrors.end_date} />
                                        </div>
                                    </div>

                                    <div className="space-y-2">
                                        <Label htmlFor="notes">Notes</Label>
                                        <textarea
                                            id="notes"
                                            name="notes"
                                            className="flex min-h-[80px] w-full rounded-md border border-input bg-background px-3 py-2 text-sm ring-offset-background placeholder:text-muted-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 disabled:cursor-not-allowed disabled:opacity-50"
                                        />
                                    </div>

                                    <div className="flex justify-end gap-3 pt-4">
                                        <Button type="button" variant="outline" onClick={() => router.back()} disabled={isLoading || success}>
                                            Cancel
                                        </Button>
                                        <Button type="submit" disabled={isLoading || success} className="bg-primary hover:bg-primary/90 text-primary-foreground">
                                            {isLoading && <Loader2 className="mr-2 h-4 w-4 animate-spin" />}
                                            {isLoading ? "Saving…" : "Save Medicine"}
                                        </Button>
                                    </div>
                                </>
                            )}
                        </form>
                    </CardContent>
                </Card>
            </div>
        </DashboardLayout>
    )
}
