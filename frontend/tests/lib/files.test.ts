import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import {
  MAX_FILE_SIZE_BYTES,
  confirmFile,
  getDownloadUrl,
  getJob,
  pollJob,
  putFileToR2,
  requestUploadUrl,
  validateFile,
} from "@/lib/files";

function jsonResponse(body: unknown, status = 200) {
  return {
    ok: status >= 200 && status < 300,
    status,
    json: async () => body,
  };
}

/** A fake XMLHttpRequest that records open/send and can be driven from tests. */
function fakeXHR() {
  const instances: any[] = [];
  class FakeXHR {
    url = "";
    method = "";
    upload: any = {};
    status = 0;
    onprogress?: () => void;
    onload?: () => void;
    onerror?: () => void;
    onabort?: () => void;
    constructor() {
      instances.push(this);
    }
    open(method: string, url: string) {
      this.method = method;
      this.url = url;
    }
    send() {
      // leave open; test drives the handlers
    }
    abort() {
      this.onabort?.();
    }
  }
  vi.stubGlobal("XMLHttpRequest", FakeXHR);
  return instances;
}

describe("lib/files validateFile", () => {
  it("accepts a pdf under the cap", () => {
    expect(validateFile(new File(["x"], "r.pdf", { type: "application/pdf" }))).toBeNull();
  });

  it("rejects an unsupported mime type", () => {
    const err = validateFile(new File(["x"], "r.gif", { type: "image/gif" }));
    expect(err).toMatch(/Unsupported file type/i);
  });

  it("rejects a file over 25 MiB", () => {
    const big = new File([new Uint8Array(MAX_FILE_SIZE_BYTES + 1)], "r.pdf", { type: "application/pdf" });
    const err = validateFile(big);
    expect(err).toMatch(/larger than 25 MB/i);
  });

  it("accepts a file exactly at the cap", () => {
    const exact = new File([new Uint8Array(MAX_FILE_SIZE_BYTES)], "r.pdf", { type: "application/pdf" });
    expect(validateFile(exact)).toBeNull();
  });
});

describe("lib/files request+confirm", () => {
  beforeEach(() => {
    window.localStorage.clear();
    window.history.replaceState(null, "", "/");
  });
  afterEach(() => vi.unstubAllGlobals());

  it("POSTs the metadata and returns the pinned shape", async () => {
    const fetchMock = vi.fn().mockResolvedValue(jsonResponse({ file_id: 7, put_url: "https://r2/x", expires_in: 300 }));
    vi.stubGlobal("fetch", fetchMock);

    const res = await requestUploadUrl("r.pdf", "application/pdf", 1234);

    expect(res).toEqual({ file_id: 7, put_url: "https://r2/x", expires_in: 300 });
    const [url, init] = fetchMock.mock.calls[0];
    expect(url.endsWith("/api/v1/files/upload-url")).toBe(true);
    expect(JSON.parse(init.body)).toEqual({ file_name: "r.pdf", mime_type: "application/pdf", size_bytes: 1234 });
  });

  it("confirmFile POSTs to /files/:id/confirm", async () => {
    const fetchMock = vi.fn().mockResolvedValue(jsonResponse({ file_id: 7, status: "active", job_id: "j-1" }));
    vi.stubGlobal("fetch", fetchMock);

    await expect(confirmFile(7)).resolves.toEqual({ file_id: 7, status: "active", job_id: "j-1" });
    expect(fetchMock.mock.calls[0][0].endsWith("/api/v1/files/7/confirm")).toBe(true);
  });

  it("getDownloadUrl returns the pinned shape", async () => {
    const fetchMock = vi.fn().mockResolvedValue(jsonResponse({ get_url: "https://r2/d", expires_in: 120 }));
    vi.stubGlobal("fetch", fetchMock);

    await expect(getDownloadUrl(7)).resolves.toEqual({ get_url: "https://r2/d", expires_in: 120 });
  });
});

describe("lib/files putFileToR2", () => {
  afterEach(() => vi.unstubAllGlobals());

  it("PUTs the blob and reports progress, then resolves", async () => {
    const instances = fakeXHR();
    const progress: number[] = [];

    const p = putFileToR2("https://r2/x", new Blob(["data"]), (pct) => progress.push(pct));
    const xhr: any = instances[0];
    expect(xhr.method).toBe("PUT");
    expect(xhr.url).toBe("https://r2/x");

    xhr.upload.onprogress?.({ lengthComputable: true, loaded: 5, total: 10 } as ProgressEvent);
    xhr.status = 201;
    xhr.onload?.();
    await p;
    expect(progress).toEqual([50]);
  });

  it("rejects on a 403 from R2", async () => {
    const instances = fakeXHR();
    const p = putFileToR2("https://r2/x", new Blob(["data"]));
    const xhr: any = instances[0];
    xhr.status = 403;
    xhr.onload?.();
    await expect(p).rejects.toThrow(/403/);
  });

  it("aborts on the signal", async () => {
    const instances = fakeXHR();
    const ctrl = new AbortController();
    const p = putFileToR2("https://r2/x", new Blob(["data"]), undefined, ctrl.signal);
    const xhr: any = instances[0];
    ctrl.abort();
    await expect(p).rejects.toMatchObject({ name: "AbortError" });
    expect(typeof xhr.onabort).toBe("function");
  });
});

describe("lib/files getJob + pollJob", () => {
  beforeEach(() => {
    window.localStorage.clear();
    window.history.replaceState(null, "", "/");
  });
  afterEach(() => {
    vi.unstubAllGlobals();
    vi.useRealTimers();
  });

  it("returns the job snapshot", async () => {
    const fetchMock = vi.fn().mockResolvedValue(
      jsonResponse({ id: "j-1", type: "ai:analyze_document", status: "queued", updated_at: "2026-11-18T00:00:00Z" })
    );
    vi.stubGlobal("fetch", fetchMock);

    await expect(getJob("j-1")).resolves.toMatchObject({ status: "queued" });
    expect(fetchMock.mock.calls[0][0].endsWith("/api/v1/jobs/j-1")).toBe(true);
  });

  it("polls with backoff until completed", async () => {
    vi.useFakeTimers();
    const states = [
      { id: "j-1", type: "t", status: "queued" },
      { id: "j-1", type: "t", status: "processing" },
      { id: "j-1", type: "t", status: "completed", result: { ok: true } },
    ];
    const fetchMock = vi.fn().mockImplementation(async () => jsonResponse(states.shift() ?? states[states.length - 1]));
    vi.stubGlobal("fetch", fetchMock);

    const ticks: string[] = [];
    const p = pollJob("j-1", { baseDelayMs: 1000, capDelayMs: 4000, onTick: (j) => ticks.push(j.status) });

    // advance through the 1s and 1.6s sleeps
    await vi.advanceTimersByTimeAsync(1000);
    await vi.advanceTimersByTimeAsync(1700);
    const job = await p;

    expect(job.status).toBe("completed");
    expect(ticks).toEqual(["queued", "processing", "completed"]);
    expect(fetchMock).toHaveBeenCalledTimes(3);
  });

  it("stops at the maxMs budget and returns the last snapshot", async () => {
    // Real (small) timers: budget is hit after a couple of short polls.
    const fetchMock = vi
      .fn()
      .mockImplementation(async () => jsonResponse({ id: "j-1", type: "t", status: "processing" }));
    vi.stubGlobal("fetch", fetchMock);

    const job = await pollJob("j-1", { baseDelayMs: 20, capDelayMs: 40, maxMs: 90 });

    expect(job.status).toBe("processing"); // budget exhausted, not failed
    expect(fetchMock.mock.calls.length).toBeGreaterThanOrEqual(2);
    expect(fetchMock.mock.calls.length).toBeLessThanOrEqual(4);
  }, 3000);

  it("throws AbortError when the signal aborts mid-poll", async () => {
    vi.useFakeTimers();
    const fetchMock = vi
      .fn()
      .mockImplementation(async () => jsonResponse({ id: "j-1", type: "t", status: "queued" }));
    vi.stubGlobal("fetch", fetchMock);

    const ctrl = new AbortController();
    const p = pollJob("j-1", { baseDelayMs: 1000, signal: ctrl.signal });
    ctrl.abort();
    await expect(p).rejects.toMatchObject({ name: "AbortError" });
  });
});
