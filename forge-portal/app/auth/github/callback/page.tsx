"use client";

import { Suspense, useEffect, useState } from "react";
import { useSearchParams, useRouter } from "next/navigation";
import { api } from "@/lib/api";
import { Loader2, CheckCircle2, XCircle } from "lucide-react";

export default function GitHubCallbackPage() {
  return (
    <Suspense
      fallback={
        <div className="flex min-h-screen items-center justify-center bg-background">
          <Loader2 className="h-12 w-12 animate-spin text-primary" />
        </div>
      }
    >
      <CallbackContent />
    </Suspense>
  );
}

function CallbackContent() {
  const searchParams = useSearchParams();
  const router = useRouter();
  const [status, setStatus] = useState<"loading" | "success" | "error">("loading");
  const [message, setMessage] = useState("正在连接 GitHub...");

  useEffect(() => {
    const code = searchParams.get("code");
    const state = searchParams.get("state");

    if (!code) {
      setStatus("error");
      setMessage("授权失败：缺少授权码");
      return;
    }

    const params = new URLSearchParams({ code });
    if (state) params.set("state", state);
    api.get(`/auth/github/callback?${params.toString()}`)
      .then(() => {
        setStatus("success");
        setMessage("GitHub 连接成功！正在跳转...");
        setTimeout(() => {
          router.push("/projects?github_connected=true");
        }, 1500);
      })
      .catch((err: unknown) => {
        setStatus("error");
        setMessage(`GitHub 授权失败：${err instanceof Error ? err.message : "未知错误"}`);
      });
  }, [searchParams, router]);

  return (
    <div className="flex min-h-screen items-center justify-center bg-background">
      <div className="flex flex-col items-center gap-4 rounded-xl border border-border bg-card p-8">
        {status === "loading" && (
          <>
            <Loader2 className="h-12 w-12 animate-spin text-primary" />
            <p className="text-lg text-muted-foreground">{message}</p>
          </>
        )}
        {status === "success" && (
          <>
            <CheckCircle2 className="h-12 w-12 text-green-500" />
            <p className="text-lg text-foreground">{message}</p>
          </>
        )}
        {status === "error" && (
          <>
            <XCircle className="h-12 w-12 text-destructive" />
            <p className="text-lg text-destructive">{message}</p>
            <button
              onClick={() => router.push("/projects")}
              className="mt-4 rounded-md bg-primary px-4 py-2 text-sm text-white hover:opacity-90 transition-opacity"
            >
              返回项目大厅
            </button>
          </>
        )}
      </div>
    </div>
  );
}
