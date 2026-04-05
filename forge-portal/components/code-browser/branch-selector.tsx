"use client";

import { GitBranch } from "lucide-react";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import type { Branch } from "@/lib/code";

interface BranchSelectorProps {
  branches: Branch[];
  currentBranch: string;
  loading?: boolean;
  onChange: (branch: string) => void;
}

export function BranchSelector({
  branches,
  currentBranch,
  loading,
  onChange,
}: BranchSelectorProps) {
  return (
    <Select value={currentBranch} onValueChange={(v) => { if (v) onChange(v); }}>
      <SelectTrigger className="w-[200px] bg-muted/50 border-border text-foreground">
        <SelectValue>
          <GitBranch className="h-3.5 w-3.5 text-accent" />
          <span className="truncate">
            {loading ? "加载中..." : currentBranch || "选择分支"}
          </span>
        </SelectValue>
      </SelectTrigger>
      <SelectContent>
        {branches.map((b) => (
          <SelectItem key={b.name} value={b.name}>
            <GitBranch className="h-3.5 w-3.5 text-muted-foreground/60" />
            {b.name}
            {b.protected && (
              <span className="ml-1 text-[10px] text-yellow-400/70">protected</span>
            )}
          </SelectItem>
        ))}
      </SelectContent>
    </Select>
  );
}
