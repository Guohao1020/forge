"use client";

import { useEffect } from "react";

export default function Error({
  error,
  reset,
}: {
  error: Error & { digest?: string };
  reset: () => void;
}) {
  useEffect(() => {
    console.error("Page error:", error);
  }, [error]);

  return (
    <div className="min-h-screen flex items-center justify-center bg-background">
      <div className="text-center max-w-md">
        <div className="text-6xl font-bold text-red-500/20 mb-4">500</div>
        <h1 className="text-2xl font-semibold text-foreground mb-2">出错了</h1>
        <p className="text-sm text-muted-foreground mb-2">
          页面加载时发生错误。请刷新重试。
        </p>
        {error.message && (
          <p className="text-xs text-red-400/70 font-mono mb-6 px-4 py-2 bg-red-500/5 rounded-lg border border-red-500/10">
            {error.message}
          </p>
        )}
        <div className="flex gap-3 justify-center">
          <button
            onClick={reset}
            className="px-6 py-2.5 bg-primary text-white rounded-lg text-sm font-medium hover:bg-primary/90 transition-colors"
          >
            重试
          </button>
          <button
            onClick={() => window.location.href = "/projects"}
            className="px-6 py-2.5 bg-white/5 border border-white/10 text-foreground rounded-lg text-sm font-medium hover:bg-white/10 transition-colors"
          >
            返回首页
          </button>
        </div>
      </div>
    </div>
  );
}
