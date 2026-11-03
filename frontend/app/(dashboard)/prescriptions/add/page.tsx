"use client";

import { useState } from "react"
import { useRouter } from "next/navigation"
import { DashboardLayout } from "@/components/layout/DashboardLayout"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card"
import { ErrorBanner, FieldError } from "@/components/ui/page-states"
import { Loader2, FileText, CheckCircle } from "lucide-react"
import { getApiUrl } from "@/lib/api"

type FieldErrors = Record<string, string>

function validatePrescription(values: Record<string, string>): FieldErrors {
    const errors: FieldErrors = {}

    if (!(values.doctor_name ?? "").trim()) {
        errors.doctor_name = "Doctor name is required."
    }
    if (!(values.diagnosis ?? "").trim()) {
        errors.diagnosis = "Diagnosis is required."
    }
    if (!(values.prescription_date ?? "").trim()) {
        errors.prescription_date = "Prescription date is required."
    }

    const rx = (values.prescription_date ?? "").trim()
    const followUp = (values.follow_up_date ?? "").trim()
    if (followUp && rx && followUp < rx) {
        errors.follow_up_date = "Follow-up date must be on or after the prescription date."
    }

    return errors
}

export default function AddPrescription() {
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

    const textareaInvalid = (field: string) =>
        fieldErrors[field]
            ? "flex min-h-[80px] w-full rounded-md border border-red-300 bg-red-50 px-3 py-2 text-sm ring-offset-background placeholder:text-muted-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-red-400 focus-visible:ring-offset-2 disabled:cursor-not-allowed disabled:opacity-50"
            : "flex min-h-[80px] w-full rounded-md border border-input bg-background px-3 py-2 text-sm ring-offset-background placeholder:text-muted-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 disabled:cursor-not-allowed disabled:opacity-50"

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

        const errors = validatePrescription(values)
        setFieldErrors(errors)
        setFormError(null)
        if (Object.keys(errors).length > 0) {
            setFormError("Please fix the highlighted fields before saving.")
            return
        }

        setIsLoading(true)

        // The Go backend binds JSON fields only; the file upload is not part of the
        // prescription record contract, so send the text fields as JSON (no CSRF).
        const body = {
            doctor_name: (values.doctor_name ?? "").trim(),
            hospital_name: (values.hospital_name ?? "").toString(),
            diagnosis: (values.diagnosis ?? "").trim(),
            prescription_date: (values.prescription_date ?? "").trim(),
            follow_up_date: (values.follow_up_date ?? "").toString() || null,
            notes: (values.notes ?? "").toString()
        }

        try {
            const response = await fetch(getApiUrl("/api/prescriptions/add"), {
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
                setTimeout(() => router.push('/prescriptions'), 800)
            } else {
                setFormError("Failed to save the prescription. Please try again.")
            }
        } catch (error: unknown) {
            if (error instanceof Error) {
                console.error("Error adding prescription:", error.message)
            } else {
                console.error("Error adding prescription:", error)
            }
            setFormError("Something went wrong while saving the prescription. Please try again.")
        } finally {
            setIsLoading(false)
        }
    }

    return (
        <DashboardLayout>
            <header className="mb-8">
                <h1 className="text-3xl font-bold tracking-tight text-primary">Add Prescription</h1>
                <p className="text-muted-foreground mt-1">Upload and track medical prescriptions</p>
            </header>

            <div className="max-w-2xl mx-auto">
                <Card className="border-border/50 bg-card/50 backdrop-blur">
                    <CardHeader>
                        <CardTitle>Prescription Details</CardTitle>
                    </CardHeader>
                    <CardContent>
                        <form method="POST" onSubmit={handleSubmit} noValidate className="space-y-6">

                            {success ? (
                                <div className="p-4 rounded-3xl bg-emerald-50 text-emerald-700 border border-emerald-100 flex items-start gap-3 text-sm">
                                    <CheckCircle className="h-5 w-5 shrink-0" />
                                    <div>
                                        <p className="font-semibold">Prescription saved</p>
                                        <p className="text-emerald-600">Taking you back to your prescriptions…</p>
                                    </div>
                                </div>
                            ) : (
                                <>
                                    {formError && <ErrorBanner>{formError}</ErrorBanner>}

                                    <div className="grid md:grid-cols-2 gap-4">
                                        <div className="space-y-2">
                                            <Label htmlFor="doctor_name">Doctor Name</Label>
                                            <div className="relative">
                                                <FileText className="absolute left-3 top-2.5 h-4 w-4 text-muted-foreground" />
                                                <Input
                                                    id="doctor_name"
                                                    name="doctor_name"
                                                    className={`pl-10 ${inputInvalid("doctor_name") || ""}`}
                                                    aria-invalid={fieldErrors.doctor_name ? true : undefined}
                                                    aria-describedby={describedBy("doctor_name")}
                                                />
                                            </div>
                                            <FieldError id="doctor_name-error" message={fieldErrors.doctor_name} />
                                        </div>
                                        <div className="space-y-2">
                                            <Label htmlFor="hospital_name">Hospital/Clinic</Label>
                                            <Input id="hospital_name" name="hospital_name" />
                                        </div>
                                    </div>

                                    <div className="space-y-2">
                                        <Label htmlFor="diagnosis">Diagnosis</Label>
                                        <textarea
                                            id="diagnosis"
                                            name="diagnosis"
                                            className={textareaInvalid("diagnosis")}
                                            aria-invalid={fieldErrors.diagnosis ? true : undefined}
                                            aria-describedby={describedBy("diagnosis")}
                                        />
                                        <FieldError id="diagnosis-error" message={fieldErrors.diagnosis} />
                                    </div>

                                    <div className="grid md:grid-cols-2 gap-4">
                                        <div className="space-y-2">
                                            <Label htmlFor="prescription_date">Date</Label>
                                            <Input
                                                id="prescription_date"
                                                name="prescription_date"
                                                type="date"
                                                className={inputInvalid("prescription_date") || "w-full"}
                                                aria-invalid={fieldErrors.prescription_date ? true : undefined}
                                                aria-describedby={describedBy("prescription_date")}
                                            />
                                            <FieldError id="prescription_date-error" message={fieldErrors.prescription_date} />
                                        </div>
                                        <div className="space-y-2">
                                            <Label htmlFor="follow_up_date">Follow-up Date</Label>
                                            <Input
                                                id="follow_up_date"
                                                name="follow_up_date"
                                                type="date"
                                                className={inputInvalid("follow_up_date") || "w-full"}
                                                aria-invalid={fieldErrors.follow_up_date ? true : undefined}
                                                aria-describedby={describedBy("follow_up_date")}
                                            />
                                            <FieldError id="follow_up_date-error" message={fieldErrors.follow_up_date} />
                                        </div>
                                    </div>

                                    <div className="space-y-2">
                                        <Label htmlFor="document">Upload Document (Optional)</Label>
                                        <div className="flex items-center gap-2">
                                            <Input
                                                id="document"
                                                name="document"
                                                type="file"
                                                accept=".pdf,.jpg,.jpeg,.png"
                                                className="cursor-pointer file:cursor-pointer"
                                            />
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
                                            {isLoading ? "Saving…" : "Save Prescription"}
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
