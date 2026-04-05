"use client";

import { useState, useEffect, useCallback } from "react";
import {
  Plus,
  Edit2,
  Trash2,
  ChevronLeft,
  ChevronRight,
} from "lucide-react";
import { Button } from "@/components/ui/button";
import { MarkdownPreview } from "@/components/markdown-preview";
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogFooter,
} from "@/components/ui/dialog";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { Textarea } from "@/components/ui/textarea";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import {
  Standard,
  listStandards,
  createStandard,
  updateStandard,
  deleteStandard,
} from "@/lib/specs";

const CATEGORIES = [
  { value: "", label: "全部分类" },
  { value: "JAVA", label: "Java" },
  { value: "SQL", label: "SQL" },
  { value: "REDIS", label: "Redis" },
  { value: "KAFKA", label: "Kafka" },
  { value: "API", label: "API" },
  { value: "SECURITY", label: "安全" },
  { value: "NAMING", label: "命名" },
  { value: "GIT", label: "Git" },
];

const SCOPES = [
  { value: "COMPANY", label: "公司级" },
  { value: "TEAM", label: "团队级" },
  { value: "PROJECT", label: "项目级" },
];

const CATEGORY_COLORS: Record<string, string> = {
  JAVA: "bg-orange-500/10 text-orange-400",
  SQL: "bg-blue-500/10 text-blue-400",
  REDIS: "bg-red-500/10 text-red-400",
  KAFKA: "bg-green-500/10 text-green-400",
  API: "bg-purple-500/10 text-purple-400",
  SECURITY: "bg-yellow-500/10 text-yellow-400",
  NAMING: "bg-cyan-500/10 text-cyan-400",
  GIT: "bg-pink-500/10 text-pink-400",
};

const SCOPE_COLORS: Record<string, string> = {
  COMPANY: "bg-accent/10 text-accent",
  TEAM: "bg-blue-500/10 text-blue-400",
  PROJECT: "bg-green-500/10 text-green-400",
};

export default function StandardsPage() {
  const [standards, setStandards] = useState<Standard[]>([]);
  const [total, setTotal] = useState(0);
  const [page, setPage] = useState(1);
  const [category, setCategory] = useState("");
  const [loading, setLoading] = useState(true);

  // Dialog state
  const [dialogOpen, setDialogOpen] = useState(false);
  const [editingStandard, setEditingStandard] = useState<Standard | null>(null);
  const [form, setForm] = useState({
    name: "",
    category: "JAVA",
    scope: "COMPANY",
    scopeId: 0,
    content: "",
  });

  const pageSize = 20;
  const totalPages = Math.ceil(total / pageSize);

  const fetchStandards = useCallback(async () => {
    setLoading(true);
    try {
      const result = await listStandards({
        category: category || undefined,
        page,
        pageSize,
      });
      setStandards(result.items || []);
      setTotal(result.total);
    } catch (err) {
      console.error("Failed to fetch standards:", err);
    } finally {
      setLoading(false);
    }
  }, [category, page]);

  useEffect(() => {
    fetchStandards();
  }, [fetchStandards]);

  const openCreate = () => {
    setEditingStandard(null);
    setForm({ name: "", category: "JAVA", scope: "COMPANY", scopeId: 0, content: "" });
    setDialogOpen(true);
  };

  const openEdit = (std: Standard) => {
    setEditingStandard(std);
    setForm({
      name: std.name,
      category: std.category,
      scope: std.scope,
      scopeId: std.scopeId,
      content: std.content,
    });
    setDialogOpen(true);
  };

  const handleSave = async () => {
    try {
      if (editingStandard) {
        await updateStandard(editingStandard.id, {
          name: form.name,
          content: form.content,
        });
      } else {
        await createStandard(form);
      }
      setDialogOpen(false);
      fetchStandards();
    } catch (err) {
      console.error("Failed to save standard:", err);
    }
  };

  const handleDelete = async (id: number) => {
    if (!confirm("确定要删除此编码规范吗？")) return;
    try {
      await deleteStandard(id);
      fetchStandards();
    } catch (err) {
      console.error("Failed to delete standard:", err);
    }
  };

  return (
    <div className="space-y-4">
      {/* Toolbar */}
      <div className="flex items-center justify-between gap-4">
        <div className="flex items-center gap-3">
          <Select value={category || "all"} onValueChange={(v) => { setCategory(!v || v === "all" ? "" : v); setPage(1); }}>
            <SelectTrigger className="w-[160px] bg-muted border-border">
              <SelectValue placeholder="全部分类" />
            </SelectTrigger>
            <SelectContent>
              {CATEGORIES.map((c) => (
                <SelectItem key={c.value || "all"} value={c.value || "all"}>
                  {c.label}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        </div>
        <Button onClick={openCreate} className="bg-accent hover:bg-accent/90 text-accent-foreground">
          <Plus className="h-4 w-4 mr-2" />
          新建规范
        </Button>
      </div>

      {/* Table */}
      <div className="bg-muted/30 border border-border rounded-lg overflow-hidden">
        <table className="w-full">
          <thead>
            <tr className="border-b border-border text-muted-foreground text-sm">
              <th className="text-left px-4 py-3 font-medium">名称</th>
              <th className="text-left px-4 py-3 font-medium">分类</th>
              <th className="text-left px-4 py-3 font-medium">作用域</th>
              <th className="text-left px-4 py-3 font-medium">版本</th>
              <th className="text-left px-4 py-3 font-medium">更新时间</th>
              <th className="text-right px-4 py-3 font-medium">操作</th>
            </tr>
          </thead>
          <tbody>
            {loading ? (
              <tr>
                <td colSpan={6} className="text-center py-12 text-muted-foreground">
                  加载中...
                </td>
              </tr>
            ) : standards.length === 0 ? (
              <tr>
                <td colSpan={6} className="text-center py-12 text-muted-foreground">
                  暂无编码规范，点击&ldquo;新建规范&rdquo;添加
                </td>
              </tr>
            ) : (
              standards.map((std) => (
                <tr
                  key={std.id}
                  className="border-b border-border hover:bg-muted/20 transition-colors"
                >
                  <td className="px-4 py-3 font-medium">{std.name}</td>
                  <td className="px-4 py-3">
                    <span
                      className={`inline-flex px-2 py-0.5 rounded text-xs font-medium ${
                        CATEGORY_COLORS[std.category] || "bg-muted text-muted-foreground"
                      }`}
                    >
                      {std.category}
                    </span>
                  </td>
                  <td className="px-4 py-3">
                    <span
                      className={`inline-flex px-2 py-0.5 rounded text-xs font-medium ${
                        SCOPE_COLORS[std.scope] || "bg-muted text-muted-foreground"
                      }`}
                    >
                      {SCOPES.find((s) => s.value === std.scope)?.label || std.scope}
                    </span>
                  </td>
                  <td className="px-4 py-3 text-muted-foreground text-sm">v{std.version}</td>
                  <td className="px-4 py-3 text-muted-foreground text-sm">
                    {new Date(std.updatedAt).toLocaleDateString("zh-CN")}
                  </td>
                  <td className="px-4 py-3 text-right">
                    <div className="flex items-center justify-end gap-1">
                      <Button
                        variant="ghost"
                        size="icon"
                        className="h-8 w-8 text-muted-foreground hover:text-foreground"
                        onClick={() => openEdit(std)}
                      >
                        <Edit2 className="h-4 w-4" />
                      </Button>
                      <Button
                        variant="ghost"
                        size="icon"
                        className="h-8 w-8 text-muted-foreground hover:text-red-500"
                        onClick={() => handleDelete(std.id)}
                      >
                        <Trash2 className="h-4 w-4" />
                      </Button>
                    </div>
                  </td>
                </tr>
              ))
            )}
          </tbody>
        </table>
      </div>

      {/* Pagination */}
      {totalPages > 1 && (
        <div className="flex items-center justify-between text-sm text-muted-foreground">
          <span>
            共 {total} 条，第 {page}/{totalPages} 页
          </span>
          <div className="flex items-center gap-2">
            <Button
              variant="ghost"
              size="icon"
              className="h-8 w-8"
              disabled={page <= 1}
              onClick={() => setPage((p) => p - 1)}
            >
              <ChevronLeft className="h-4 w-4" />
            </Button>
            <Button
              variant="ghost"
              size="icon"
              className="h-8 w-8"
              disabled={page >= totalPages}
              onClick={() => setPage((p) => p + 1)}
            >
              <ChevronRight className="h-4 w-4" />
            </Button>
          </div>
        </div>
      )}

      {/* Create/Edit Dialog */}
      <Dialog open={dialogOpen} onOpenChange={setDialogOpen}>
        <DialogContent className="bg-background border-border text-foreground max-w-[75vw] sm:max-w-[75vw] max-h-[85vh] flex flex-col">
          <DialogHeader>
            <DialogTitle>
              {editingStandard ? "编辑编码规范" : "新建编码规范"}
            </DialogTitle>
          </DialogHeader>
          <div className="space-y-4 py-2">
            <div className="space-y-2">
              <Label>名称</Label>
              <Input
                value={form.name}
                onChange={(e) => setForm({ ...form, name: e.target.value })}
                placeholder="如：Java 编码规范"
                className="bg-muted border-border"
              />
            </div>
            {!editingStandard && (
              <div className="grid grid-cols-2 gap-4">
                <div className="space-y-2">
                  <Label>分类</Label>
                  <Select
                    value={form.category}
                    onValueChange={(v) => v && setForm({ ...form, category: v })}
                  >
                    <SelectTrigger className="bg-muted border-border">
                      <SelectValue />
                    </SelectTrigger>
                    <SelectContent>
                      {CATEGORIES.filter((c) => c.value).map((c) => (
                        <SelectItem key={c.value} value={c.value}>
                          {c.label}
                        </SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                </div>
                <div className="space-y-2">
                  <Label>作用域</Label>
                  <Select
                    value={form.scope}
                    onValueChange={(v) => v && setForm({ ...form, scope: v })}
                  >
                    <SelectTrigger className="bg-muted border-border">
                      <SelectValue />
                    </SelectTrigger>
                    <SelectContent>
                      {SCOPES.map((s) => (
                        <SelectItem key={s.value} value={s.value}>
                          {s.label}
                        </SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                </div>
              </div>
            )}
            {/* Split-pane: Editor + Preview */}
            <div>
              <div className="flex items-center justify-between mb-2">
                <Label>规范内容（Markdown）</Label>
                <span className="text-xs text-muted-foreground">左侧编辑 · 右侧预览</span>
              </div>
              <div className="grid grid-cols-2 gap-4" style={{ height: "calc(85vh - 260px)" }}>
                <Textarea
                  value={form.content}
                  onChange={(e) => setForm({ ...form, content: e.target.value })}
                  placeholder="输入编码规范内容，支持 Markdown 格式..."
                  className="bg-[#FAFAFA] border-border font-mono text-sm resize-none overflow-y-auto"
                  style={{ height: "100%", minHeight: "unset", fieldSizing: "fixed" }}
                />
                <div className="border border-border rounded-lg bg-[#FAFAFA] p-4 overflow-y-auto">
                  <MarkdownPreview content={form.content} />
                </div>
              </div>
            </div>
          </div>
          <DialogFooter>
            <Button
              variant="ghost"
              onClick={() => setDialogOpen(false)}
              className="text-muted-foreground"
            >
              取消
            </Button>
            <Button
              onClick={handleSave}
              className="bg-accent hover:bg-accent/90 text-accent-foreground"
              disabled={!form.name || !form.content}
            >
              {editingStandard ? "保存修改" : "创建"}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  );
}
