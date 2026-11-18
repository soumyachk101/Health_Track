/**
 * [INPUT]: depends on FilesUpload from @/components/FilesUpload; depends on
 *          DashboardLayout, Card, and page-states from the shared layout/UI kit.
 * [OUTPUT]: the /files page of the (dashboard) group — the only surface where
 *          patients upload medical documents; uploads land in R2 and are
 *          verified by the asynq worker (no local disk writes).
 * [POS]: sibling of /medicines and /health-records. It holds exactly one card
 *          (the uploader) plus a short "what happens next" note, because the
 *          rest of the flow is worker-side and surfaced through the uploader's
 *          job-polling states.
 * [PROTOCOL]: Update this header on change, then check CLAUDE.md
 */
"use client";

import { FileText, Info } from "lucide-react";
import { DashboardLayout } from "@/components/layout/DashboardLayout";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { FilesUpload } from "@/components/FilesUpload";

export default function FilesPage() {
    return (
        <DashboardLayout>
            <header className="mb-8">
                <h1 className="text-3xl font-bold tracking-tight text-primary">My Files</h1>
                <p className="text-muted-foreground mt-1">Upload medical records, prescriptions, and reports</p>
            </header>

            <div className="grid gap-6 lg:grid-cols-3">
                <Card className="lg:col-span-2">
                    <CardHeader>
                        <CardTitle className="flex items-center gap-2 text-base font-semibold">
                            <FileText className="h-4 w-4 text-primary" aria-hidden="true" />
                            Upload a document
                        </CardTitle>
                    </CardHeader>
                    <CardContent>
                        <FilesUpload
                            inputId="files-page-upload"
                            hint="PDF, PNG, JPEG, or WebP · up to 25 MB. The file is stored in secure object storage and verified by the server before it is attached to your records."
                        />
                    </CardContent>
                </Card>

                <Card>
                    <CardHeader>
                        <CardTitle className="flex items-center gap-2 text-base font-semibold">
                            <Info className="h-4 w-4 text-primary" aria-hidden="true" />
                            What happens next
                        </CardTitle>
                    </CardHeader>
                    <CardContent className="space-y-3 text-sm text-muted-foreground">
                        <p>1. Your browser uploads the file directly to secure storage.</p>
                        <p>2. The server verifies the upload and attaches it to your medical files.</p>
                        <p>3. If verification is still running when the wait ends, the page says so — the file is not lost.</p>
                        <p className="text-xs">Uploads are encrypted in transit. Downloads use short-lived links that expire.</p>
                    </CardContent>
                </Card>
            </div>
        </DashboardLayout>
    );
}
