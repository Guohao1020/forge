"use client";

import { useAuth } from "@/lib/auth";
import { LogOut, User } from "lucide-react";

export function Topbar() {
  const { user, logout } = useAuth();

  return (
    <header className="h-14 flex items-center justify-end px-6 border-b border-[var(--border)] bg-[var(--card)]">
      <div className="flex items-center gap-4">
        <div className="flex items-center gap-2 text-sm text-[var(--muted-foreground)]">
          <User size={16} />
          <span>{user?.display_name || user?.username}</span>
        </div>
        <button
          onClick={logout}
          className="flex items-center gap-1.5 text-sm text-[var(--muted-foreground)] transition-colors hover:text-[var(--destructive)]"
        >
          <LogOut size={16} />
          登出
        </button>
      </div>
    </header>
  );
}
