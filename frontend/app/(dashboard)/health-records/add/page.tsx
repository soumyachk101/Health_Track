"use client";

import { useState } from "react"
import { useRouter } from "next/navigation"
import { DashboardLayout } from "@/components/layout/DashboardLayout"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Card, CardContent, CardHeader, CardTitle, CardDescription } from "@/components/ui/card"
import { ErrorBanner, FieldError } from "@/components/ui/page-states"
import { Loader2, Activity, CheckCircle } from "lucide-react"
import { getApiUrl } from "@/lib/api"

type FieldErrors = Record<string, string>

// Sensible physiological ranges — only enforced when a value is provided,
// since the backend treats every vital as optional.
function validateHealthForm(values: Record<string, string>): FieldErrors {
    const errors: FieldErrors = {}
    const num = (v: string) => (v.trim() === "" ? null : Number(v))

    const inRange = (field: string, v: number | null, min: number, max: number, label: string, message: string) => {
        if (v !== null && (Number.isNaN(v) || v < min || v > max)) {
            errors[field] = message
        }
    }

    const sys = num(values.blood_pressure_systolic ?? "")
    const dia = num(values.blood_pressure_diastolic ?? "")
    const sugar = num(values.blood_sugar ?? "")
    const weight = num(values.weight ?? "")
    const rate = num(values.heart_rate ?? "")
    const temp = num(values.temperature ?? "")
    const oxy = num(values.oxygen_level ?? "")

    inRange("blood_pressure_systolic", sys, 60, 260, "BP Systolic", "BP systolic must be between 60 and 260 mmHg.")
    inRange("blood_pressure_diastolic", dia, 30, 160, "BP Diastolic", "BP diastolic must be between 30 and 160 mmHg.")
    inRange("heart_rate", rate, 30, 220, "Heart rate", "Heart rate must be between 30 and 220 bpm.")
    inRange("temperature", temp, 30, 45, "Temperature", "Temperature must be between 30 and 45 °F.")
    inRange("oxygen_level", oxy, 0, 100, "Oxygen", "Oxygen level must be between 0 and 100%.")

    if (sugar !== null && (Number.isNaN(sugar) || sugar <= 0)) {
        errors.blood_sugar = "Blood sugar must be greater than 0 mg/dL."
    }
    if (weight !== null && (Number.isNaN(weight) || weight <= 0)) {
        errors.weight = "Weight must be greater than 0 kg."
    }

    // Diastolic should be lower than systolic when both are present.
    if (sys !== null && dia !== null && !Number.isNaN(sys) && !Number.isNaN(dia) && dia >= sys) {
        errors.blood_pressure_diastolic = errors.blood_pressure_diastolic || "Diastolic should be lower than systolic."
    }

    return errors
}

export default function AddHealthRecord() {
    const [isLoading, setIsLoading] = useState(false)
    const [fieldErrors, setFieldErrors] = useState<FieldErrors>({})
    const [formError, setFormError] = useState<string | null>(null)
    const [success, setSuccess] = useState(false)
    const router = useRouter()

    // aria-describedby wiring per field: the (hidden) FieldError element id.
    const describedBy = (field: string, extra: string[] = []) => {
        const ids = extra.filter(Boolean)
        if (fieldErrors[field]) ids.push(`${field}-error`)
        return ids.join(" ") || undefined
    }

    const inputInvalid = (field: string) =>
        fieldErrors[field] ? "border-red-300 shadow-skeuo-inset-md bg-red-50" : undefined

    const handleSubmit = async (e: React.FormEvent<HTMLFormElement>) => {
        e.preventDefault()
        if (isLoading || success) return // prevent duplicate submits

        const token = localStorage.getItem('token')
        if (!token) {
            router.push('/login')
            return
        }

        const formData = new FormData(e.currentTarget)
        const raw = Object.fromEntries(formData.entries())
        const values: Record<string, string> = {}
        for (const [k, v] of Object.entries(raw)) {
            values[k] = (v ?? "").toString()
        }

        const errors = validateHealthForm(values)
        setFieldErrors(errors)
        setFormError(null)
        if (Object.keys(errors).length > 0) {
            setFormError("Please fix the highlighted fields before saving.")
            return
        }

        setIsLoading(true)

        const numOr = (v: string | undefined) => {
            const s = (v ?? "").trim()
            return s === "" ? null : Number(s)
        }
        // The Go backend binds these as numeric JSON fields; send numbers (or null), not strings.
        const body = {
            blood_pressure_systolic: numOr(values.blood_pressure_systolic),
            blood_pressure_diastolic: numOr(values.blood_pressure_diastolic),
            blood_sugar: numOr(values.blood_sugar),
            weight: numOr(values.weight),
            heart_rate: numOr(values.heart_rate),
            temperature: numOr(values.temperature),
            oxygen_level: numOr(values.oxygen_level),
            notes: values.notes ?? ""
        }

        try {
            const response = await fetch(getApiUrl("/api/health-track/add"), {
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
                setTimeout(() => router.push('/health-track'), 800)
            } else {
                setFormError("Failed to save your record. Please try again.")
            }
        } catch (error: unknown) {
            if (error instanceof Error) {
                console.error("Error adding record:", error.message)
            } else {
                console.error("Error adding record:", error)
            }
            setFormError("Something went wrong while saving your record. Please try again.")
        } finally {
            setIsLoading(false)
        }
    }

    return (
        <DashboardLayout>
            <header className="mb-8">
                <h1 className="text-3xl font-bold tracking-tight text-primary">Add Health Record</h1>
                <p className="text-muted-foreground mt-1">Record your daily vitals</p>
            </header>

            <div className="max-w-2xl mx-auto">
                <Card className="border-border/50 bg-card/50 backdrop-blur">
                    <CardHeader>
                        <CardTitle>Vital Signs</CardTitle>
                        <CardDescription>Enter your latest health readings</CardDescription>
                    </CardHeader>
                    <CardContent>
                        <form method="POST" onSubmit={handleSubmit} noValidate className="space-y-6">

                            {success ? (
                                <div className="p-4 rounded-3xl bg-emerald-50 text-emerald-700 border border-emerald-100 flex items-start gap-3 text-sm">
                                    <CheckCircle className="h-5 w-5 shrink-0" />
                                    <div>
                                        <p className="font-semibold">Record saved</p>
                                        <p className="text-emerald-600">Taking you back to your health track…</p>
                                    </div>
                                </div>
                            ) : (
                                <>
                                    {formError && <ErrorBanner>{formError}</ErrorBanner>}

                                    <div className="grid md:grid-cols-2 gap-4">
                                        <div className="space-y-2">
                                            <Label htmlFor="blood_pressure_systolic">BP Systolic</Label>
                                            <div className="relative">
                                                <Activity className="absolute left-3 top-2.5 h-4 w-4 text-muted-foreground" />
                                                <Input
                                                    id="blood_pressure_systolic"
                                                    name="blood_pressure_systolic"
                                                    type="number"
                                                    placeholder="120"
                                                    className={inputInvalid("blood_pressure_systolic")}
                                                    aria-invalid={fieldErrors.blood_pressure_systolic ? true : undefined}
                                                    aria-describedby={describedBy("blood_pressure_systolic")}
                                                />
                                            </div>
                                            <FieldError id="blood_pressure_systolic-error" message={fieldErrors.blood_pressure_systolic} />
                                        </div>
                                        <div className="space-y-2">
                                            <Label htmlFor="blood_pressure_diastolic">BP Diastolic</Label>
                                            <Input
                                                id="blood_pressure_diastolic"
                                                name="blood_pressure_diastolic"
                                                type="number"
                                                placeholder="80"
                                                className={inputInvalid("blood_pressure_diastolic")}
                                                aria-invalid={fieldErrors.blood_pressure_diastolic ? true : undefined}
                                                aria-describedby={describedBy("blood_pressure_diastolic")}
                                            />
                                            <FieldError id="blood_pressure_diastolic-error" message={fieldErrors.blood_pressure_diastolic} />
                                        </div>
                                    </div>

                                    <div className="grid md:grid-cols-2 gap-4">
                                        <div className="space-y-2">
                                            <Label htmlFor="blood_sugar">Blood Sugar (mg/dL)</Label>
                                            <Input
                                                id="blood_sugar"
                                                name="blood_sugar"
                                                type="number"
                                                step="0.01"
                                                className={inputInvalid("blood_sugar")}
                                                aria-invalid={fieldErrors.blood_sugar ? true : undefined}
                                                aria-describedby={describedBy("blood_sugar")}
                                            />
                                            <FieldError id="blood_sugar-error" message={fieldErrors.blood_sugar} />
                                        </div>
                                        <div className="space-y-2">
                                            <Label htmlFor="weight">Weight (kg)</Label>
                                            <Input
                                                id="weight"
                                                name="weight"
                                                type="number"
                                                step="0.01"
                                                className={inputInvalid("weight")}
                                                aria-invalid={fieldErrors.weight ? true : undefined}
                                                aria-describedby={describedBy("weight")}
                                            />
                                            <FieldError id="weight-error" message={fieldErrors.weight} />
                                        </div>
                                    </div>

                                    <div className="grid md:grid-cols-3 gap-4">
                                        <div className="space-y-2">
                                            <Label htmlFor="heart_rate">Heart Rate (bpm)</Label>
                                            <Input
                                                id="heart_rate"
                                                name="heart_rate"
                                                type="number"
                                                className={inputInvalid("heart_rate")}
                                                aria-invalid={fieldErrors.heart_rate ? true : undefined}
                                                aria-describedby={describedBy("heart_rate")}
                                            />
                                            <FieldError id="heart_rate-error" message={fieldErrors.heart_rate} />
                                        </div>
                                        <div className="space-y-2">
                                            <Label htmlFor="temperature">Temp (&deg;F)</Label>
                                            <Input
                                                id="temperature"
                                                name="temperature"
                                                type="number"
                                                step="0.1"
                                                className={inputInvalid("temperature")}
                                                aria-invalid={fieldErrors.temperature ? true : undefined}
                                                aria-describedby={describedBy("temperature")}
                                            />
                                            <FieldError id="temperature-error" message={fieldErrors.temperature} />
                                        </div>
                                        <div className="space-y-2">
                                            <Label htmlFor="oxygen_level">Oxygen (%)</Label>
                                            <Input
                                                id="oxygen_level"
                                                name="oxygen_level"
                                                type="number"
                                                className={inputInvalid("oxygen_level")}
                                                aria-invalid={fieldErrors.oxygen_level ? true : undefined}
                                                aria-describedby={describedBy("oxygen_level")}
                                            />
                                            <FieldError id="oxygen_level-error" message={fieldErrors.oxygen_level} />
                                        </div>
                                    </div>

                                    <div className="space-y-2">
                                        <Label htmlFor="notes">Notes</Label>
                                        <textarea
                                            id="notes"
                                            name="notes"
                                            className="flex min-h-[80px] w-full rounded-md border border-input bg-background px-3 py-2 text-sm ring-offset-background placeholder:text-muted-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 disabled:cursor-not-allowed disabled:opacity-50"
                                            placeholder="Any additional observations..."
                                        />
                                    </div>

                                    <div className="flex justify-end gap-3 pt-4">
                                        <Button type="button" variant="outline" onClick={() => router.back()} disabled={isLoading || success}>
                                            Cancel
                                        </Button>
                                        <Button type="submit" disabled={isLoading || success} className="bg-primary hover:bg-primary/90 text-primary-foreground">
                                            {isLoading && <Loader2 className="mr-2 h-4 w-4 animate-spin" />}
                                            {isLoading ? "Saving…" : "Save Record"}
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
