"use client";

import { useEffect, useState, useMemo } from "react";
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogFooter,
} from "@/components/ui/dialog";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Badge } from "@/components/ui/badge";
import { api } from "@/lib/api";
import { Search, Loader2, Lock, GitFork, Star } from "lucide-react";

interface GitHubRepo {
  id: number;
  owner: string;
  name: string;
  full_name: string;
  description: string;
  html_url: string;
  clone_url: string;
  default_branch: string;
  language: string;
  private: boolean;
  fork: boolean;
  star_count: number;
  updated_at: string;
}

interface ImportReposDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  onImported: () => void;
}

export function ImportReposDialog({
  open,
  onOpenChange,
  onImported,
}: ImportReposDialogProps) {
  const [repos, setRepos] = useState<GitHubRepo[]>([]);
  const [loading, setLoading] = useState(false);
  const [importing, setImporting] = useState(false);
  const [search, setSearch] = useState("");
  const [selected, setSelected] = useState<Set<number>>(new Set());
  const [error, setError] = useState("");
  const [result, setResult] = useState("");

  useEffect(() => {
    if (!open) return;
    setLoading(true);
    setError("");
    setResult("");
    setSelected(new Set());
    api.get<GitHubRepo[]>("/github/repos")
      .then((data) => setRepos(data ?? []))
      .catch((err: unknown) => setError(err instanceof Error ? err.message : "获取仓库失败"))
      .finally(() => setLoading(false));
  }, [open]);

  const filtered = useMemo(() => {
    if (!search) return repos;
    const q = search.toLowerCase();
    return repos.filter(
      (r) =>
        r.full_name.toLowerCase().includes(q) ||
        r.description?.toLowerCase().includes(q) ||
        r.language?.toLowerCase().includes(q)
    );
  }, [repos, search]);

  // Group repos by owner
  const grouped = useMemo(() => {
    const map = new Map<string, GitHubRepo[]>();
    for (const r of filtered) {
      const list = map.get(r.owner) ?? [];
      list.push(r);
      map.set(r.owner, list);
    }
    return map;
  }, [filtered]);

  function toggle(id: number) {
    setSelected((prev) => {
      const next = new Set(prev);
      if (next.has(id)) next.delete(id);
      else next.add(id);
      return next;
    });
  }

  async function handleImport() {
    const items = repos
      .filter((r) => selected.has(r.id))
      .map((r) => ({
        full_name: r.full_name,
        name: r.name,
        description: r.description,
        html_url: r.html_url,
        clone_url: r.clone_url,
        default_branch: r.default_branch,
        language: r.language,
      }));

    setImporting(true);
    setError("");
    try {
      const data = await api.post<{
        imported: number;
        skipped: number;
        errors?: string[];
      }>("/projects/import", { repos: items });
      setResult(`导入 ${data.imported} 个，跳过 ${data.skipped} 个`);
      if (data.imported > 0) {
        setTimeout(() => {
          onOpenChange(false);
          onImported();
        }, 1500);
      }
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : "导入失败");
    } finally {
      setImporting(false);
    }
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="bg-card border-border text-foreground max-w-lg">
        <DialogHeader>
          <DialogTitle>导入 GitHub 仓库</DialogTitle>
        </DialogHeader>

        {/* Search */}
        <div className="relative">
          <Search className="absolute left-3 top-1/2 -translate-y-1/2 h-4 w-4 text-muted-foreground" />
          <Input
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            placeholder="搜索仓库..."
            className="pl-9 bg-input border-border"
          />
        </div>

        {/* Repo list */}
        <div className="max-h-[400px] overflow-y-auto space-y-4 -mx-1 px-1">
          {loading ? (
            <div className="flex items-center justify-center py-12">
              <Loader2 className="h-6 w-6 animate-spin text-primary" />
              <span className="ml-2 text-sm text-muted-foreground">加载仓库列表...</span>
            </div>
          ) : error && repos.length === 0 ? (
            <p className="text-center text-sm text-destructive py-8">{error}</p>
          ) : filtered.length === 0 ? (
            <p className="text-center text-sm text-muted-foreground py-8">
              {search ? "没有匹配的仓库" : "没有可用仓库"}
            </p>
          ) : (
            Array.from(grouped.entries()).map(([owner, ownerRepos]) => (
              <div key={owner}>
                <p className="text-xs font-medium text-muted-foreground mb-2 sticky top-0 bg-card py-1">
                  {owner}
                </p>
                <div className="space-y-1">
                  {ownerRepos.map((repo) => (
                    <label
                      key={repo.id}
                      className={`flex items-center gap-3 rounded-lg border p-3 cursor-pointer transition-colors ${
                        selected.has(repo.id)
                          ? "border-primary/50 bg-primary/5"
                          : "border-border hover:border-primary/30"
                      }`}
                    >
                      <input
                        type="checkbox"
                        checked={selected.has(repo.id)}
                        onChange={() => toggle(repo.id)}
                        className="rounded border-border accent-[#8B5CF6]"
                      />
                      <div className="flex-1 min-w-0">
                        <div className="flex items-center gap-1.5">
                          <span className="text-sm font-medium truncate">{repo.name}</span>
                          {repo.private && <Lock className="h-3 w-3 text-muted-foreground shrink-0" />}
                          {repo.fork && <GitFork className="h-3 w-3 text-muted-foreground shrink-0" />}
                        </div>
                        {repo.description && (
                          <p className="text-xs text-muted-foreground truncate mt-0.5">
                            {repo.description}
                          </p>
                        )}
                      </div>
                      <div className="flex items-center gap-2 shrink-0">
                        {repo.language && (
                          <Badge variant="secondary" className="text-xs py-0">
                            {repo.language}
                          </Badge>
                        )}
                        {repo.star_count > 0 && (
                          <span className="flex items-center gap-0.5 text-xs text-muted-foreground">
                            <Star className="h-3 w-3" />
                            {repo.star_count}
                          </span>
                        )}
                      </div>
                    </label>
                  ))}
                </div>
              </div>
            ))
          )}
        </div>

        {error && repos.length > 0 && (
          <p className="text-sm text-destructive">{error}</p>
        )}
        {result && <p className="text-sm text-green-500">{result}</p>}

        <DialogFooter>
          <Button variant="ghost" onClick={() => onOpenChange(false)} disabled={importing}>
            取消
          </Button>
          <Button onClick={handleImport} disabled={selected.size === 0 || importing}>
            {importing ? "导入中..." : `导入选中 (${selected.size})`}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
