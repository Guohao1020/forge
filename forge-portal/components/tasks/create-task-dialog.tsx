"use client";

import { useState } from "react";
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogFooter,
} from "@/components/ui/dialog";
import { Button } from "@/components/ui/button";
import { Textarea } from "@/components/ui/textarea";
import { createTask } from "@/lib/tasks";

interface CreateTaskDialogProps {
  projectId: string;
  open: boolean;
  onOpenChange: (open: boolean) => void;
  onCreated: () => void;
}

export function CreateTaskDialog({
  projectId,
  open,
  onOpenChange,
  onCreated,
}: CreateTaskDialogProps) {
  const [requirement, setRequirement] = useState("");
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState("");

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    if (loading) return;
    if (!requirement.trim()) {
      setError("请输入需求描述");
      return;
    }
    setLoading(true);
    setError("");
    try {
      await createTask(projectId, requirement.trim());
      setRequirement("");
      onOpenChange(false);
      onCreated();
    } catch (err: unknown) {
      console.error("[CreateTask] error:", err);
      setError(err instanceof Error ? err.message : "创建失败");
    } finally {
      setLoading(false);
    }
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="bg-card border-border text-foreground">
        <DialogHeader>
          <DialogTitle>新建任务</DialogTitle>
        </DialogHeader>
        <form onSubmit={handleSubmit} className="space-y-4">
          <div className="space-y-2">
            <label className="text-sm font-medium">需求描述 *</label>
            <Textarea
              value={requirement}
              onChange={(e) => setRequirement(e.target.value)}
              placeholder="用自然语言描述你的需求，例如：实现用户注册功能，支持手机号和邮箱注册..."
              className="bg-input border-border resize-none min-h-[120px]"
              autoFocus
            />
          </div>
          {error && <p className="text-sm text-destructive">{error}</p>}
          <DialogFooter>
            <Button variant="ghost" type="button" onClick={() => onOpenChange(false)} disabled={loading}>
              取消
            </Button>
            <Button type="submit" disabled={loading}>
              {loading ? "创建中..." : "创建任务"}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}
