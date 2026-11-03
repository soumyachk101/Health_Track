"use client";

import Link from "next/link";
import { Activity, Compass, LogIn, Home } from "lucide-react";

export default function NotFound() {
  return (
    <main className="min-h-screen bg-[#EFF6FF] text-slate-700 font-sans flex items-center justify-center px-4 py-16 relative overflow-x-hidden">
      <div className="bg-texture"></div>

      <div className="relative z-10 w-full max-w-lg">
        <div className="card-skeuo rounded-3xl p-8 md:p-10 text-center space-y-6">
          <div className="mx-auto w-20 h-20 rounded-full bg-[#EFF6FF] shadow-skeuo-md flex items-center justify-center text-teal-600 border border-white/50">
            <Compass className="w-10 h-10" aria-hidden="true" />
          </div>

          <h1 className="text-4xl md:text-5xl font-black tracking-tight text-slate-800">
            404 <span className="text-teal-500">·</span> Page Not Found
          </h1>

          <p className="text-slate-500 leading-relaxed">
            This page seems to have gone missing — it may have been moved,
            renamed, or never existed in the first place. The rest of
            HealthTrack+ is still up and running.
          </p>

          <div className="flex flex-col sm:flex-row gap-3 sm:gap-4 justify-center items-center pt-2">
            <Link href="/" className="w-full sm:w-auto">
              <button className="btn-skeuo-primary text-sm px-6 py-3 flex items-center justify-center gap-2">
                <Home className="w-4 h-4" aria-hidden="true" />
                Go to Home
              </button>
            </Link>
            <Link href="/login" className="w-full sm:w-auto">
              <button className="btn-skeuo text-sm px-6 py-3 flex items-center justify-center gap-2">
                <LogIn className="w-4 h-4" aria-hidden="true" />
                Log in
              </button>
            </Link>
          </div>
        </div>

        <p className="flex items-center justify-center gap-2 text-xs text-slate-400 mt-8">
          <Activity className="w-3 h-3" aria-hidden="true" />
          HealthTrack+ — tactile health management
        </p>
      </div>
    </main>
  );
}
