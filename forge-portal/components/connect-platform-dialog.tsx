"use client";

import { useState } from "react";
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Button } from "@/components/ui/button";
import { api } from "@/lib/api";
import { Loader2 } from "lucide-react";
import { GitHubIcon } from "@/components/icons";

interface ConnectPlatformDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
}

export function ConnectPlatformDialog({
  open,
  onOpenChange,
}: ConnectPlatformDialogProps) {
  const [loading, setLoading] = useState(false);

  async function handleConnectGitHub() {
    setLoading(true);
    try {
      const data = await api.get<{ authorize_url: string }>("/auth/github/authorize");
      window.location.href = data.authorize_url;
    } catch {
      setLoading(false);
    }
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="bg-card border-border text-foreground">
        <DialogHeader>
          <DialogTitle>接入代码平台</DialogTitle>
        </DialogHeader>
        <p className="text-sm text-muted-foreground mb-4">
          授权后系统将自动同步仓库列表，你可以选择仓库导入为 Forge 项目。
        </p>
        <div className="space-y-3">
          <button
            onClick={handleConnectGitHub}
            disabled={loading}
            className="flex w-full items-center gap-4 rounded-xl border border-border bg-secondary/50 p-4 transition-colors hover:border-primary/50 hover:bg-secondary disabled:opacity-50"
          >
            <div className="flex h-10 w-10 items-center justify-center rounded-lg bg-[#24292e]">
              <GitHubIcon className="h-5 w-5 text-white" />
            </div>
            <div className="text-left flex-1">
              <p className="text-sm font-medium text-foreground">GitHub</p>
              <p className="text-xs text-muted-foreground">
                连接 GitHub 账户，同步公开和私有仓库
              </p>
            </div>
            {loading && <Loader2 className="h-4 w-4 animate-spin text-muted-foreground" />}
          </button>

          {/* Future: Codeup, GitLab, etc. */}
          <div className="flex w-full items-center gap-4 rounded-xl border border-border/50 bg-secondary/20 p-4 opacity-50 cursor-not-allowed">
            <div className="flex h-10 w-10 items-center justify-center rounded-lg bg-muted">
              <span className="text-xs font-medium text-muted-foreground">Ali</span>
            </div>
            <div className="text-left flex-1">
              <p className="text-sm font-medium text-muted-foreground">Codeup</p>
              <p className="text-xs text-muted-foreground">阿里云效 Codeup（即将支持）</p>
            </div>
          </div>
        </div>

        <div className="mt-2">
          <Button variant="ghost" className="w-full" onClick={() => onOpenChange(false)}>
            取消
          </Button>
        </div>
      </DialogContent>
    </Dialog>
  );
}
