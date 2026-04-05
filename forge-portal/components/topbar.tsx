"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import { useRouter } from "next/navigation";
import { useAuth } from "@/lib/auth";
import { LogOut, User, Search } from "lucide-react";
import { api } from "@/lib/api";

interface SearchResultItem {
  type: string;
  id: number;
  projectId?: number;
  title: string;
  description?: string;
  status?: string;
  url: string;
}

export function Topbar() {
  const { user, logout } = useAuth();
  const router = useRouter();
  const [searchQuery, setSearchQuery] = useState("");
  const [results, setResults] = useState<SearchResultItem[]>([]);
  const [showResults, setShowResults] = useState(false);
  const [searching, setSearching] = useState(false);
  const searchRef = useRef<HTMLDivElement>(null);
  const inputRef = useRef<HTMLInputElement>(null);

  const doSearch = useCallback(async (q: string) => {
    if (q.length < 2) {
      setResults([]);
      return;
    }
    setSearching(true);
    try {
      const res = await api.get<{ results: SearchResultItem[] }>(`/search?q=${encodeURIComponent(q)}`);
      setResults(res.results);
    } catch {
      setResults([]);
    } finally {
      setSearching(false);
    }
  }, []);

  useEffect(() => {
    const timer = setTimeout(() => doSearch(searchQuery), 300);
    return () => clearTimeout(timer);
  }, [searchQuery, doSearch]);

  // Close on click outside
  useEffect(() => {
    const handleClick = (e: MouseEvent) => {
      if (searchRef.current && !searchRef.current.contains(e.target as Node)) {
        setShowResults(false);
      }
    };
    document.addEventListener("mousedown", handleClick);
    return () => document.removeEventListener("mousedown", handleClick);
  }, []);

  // Keyboard shortcut: Cmd/Ctrl+K to focus search
  useEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      if ((e.metaKey || e.ctrlKey) && e.key === "k") {
        e.preventDefault();
        inputRef.current?.focus();
        setShowResults(true);
      }
      if (e.key === "Escape") {
        setShowResults(false);
        inputRef.current?.blur();
      }
    };
    document.addEventListener("keydown", handleKeyDown);
    return () => document.removeEventListener("keydown", handleKeyDown);
  }, []);

  return (
    <header className="h-14 flex items-center justify-between px-6 border-b border-[var(--border)] bg-[var(--card)]">
      {/* Search */}
      <div ref={searchRef} className="relative w-80">
        <div className="flex items-center gap-2 bg-white/5 border border-white/10 rounded-lg px-3 py-1.5">
          <Search size={14} className="text-[var(--muted-foreground)] shrink-0" />
          <input
            ref={inputRef}
            type="text"
            value={searchQuery}
            onChange={(e) => {
              setSearchQuery(e.target.value);
              setShowResults(true);
            }}
            onFocus={() => setShowResults(true)}
            placeholder="搜索项目和任务..."
            className="bg-transparent text-sm text-[var(--foreground)] placeholder:text-white/30 focus:outline-none w-full"
          />
          <kbd className="hidden sm:inline-flex items-center gap-0.5 px-1.5 py-0.5 text-[10px] text-white/30 bg-white/5 rounded border border-white/10 shrink-0">
            <span className="text-[9px]">⌘</span>K
          </kbd>
          {searching && (
            <div className="w-3 h-3 border-2 border-primary/30 border-t-primary rounded-full animate-spin shrink-0" />
          )}
        </div>

        {/* Results dropdown */}
        {showResults && results.length > 0 && (
          <div className="absolute top-full left-0 right-0 mt-1 bg-[var(--card)] border border-[var(--border)] rounded-lg shadow-lg z-50 max-h-80 overflow-auto">
            {results.map((r) => (
              <button
                key={`${r.type}-${r.id}`}
                onClick={() => {
                  router.push(r.url);
                  setShowResults(false);
                  setSearchQuery("");
                }}
                className="w-full text-left px-3 py-2 hover:bg-white/5 transition-colors border-b border-white/5 last:border-0"
              >
                <div className="flex items-center gap-2">
                  <span className={`px-1.5 py-0.5 rounded text-[9px] uppercase font-medium ${
                    r.type === "project"
                      ? "bg-primary/10 text-primary"
                      : "bg-blue-500/10 text-blue-400"
                  }`}>
                    {r.type === "project" ? "项目" : "任务"}
                  </span>
                  <span className="text-sm text-[var(--foreground)] truncate">{r.title}</span>
                </div>
                {r.description && (
                  <p className="text-xs text-[var(--muted-foreground)] mt-0.5 truncate pl-9">{r.description}</p>
                )}
              </button>
            ))}
          </div>
        )}
      </div>

      {/* User */}
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
