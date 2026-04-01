"use client";

import { useState, useEffect, useCallback } from "react";
import {
  Plus,
  Edit2,
  Trash2,
  ChevronLeft,
  ChevronRight,
  Star,
  X,
} from "lucide-react";
import { Button } from "@/components/ui/button";
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
  PromptTemplate,
  listPromptTemplates,
  createPromptTemplate,
  updatePromptTemplate,
  deletePromptTemplate,
} from "@/lib/specs";

const PURPOSES = [
  { value: "", label: "全部用途" },
  { value: "requirement-analysis", label: "需求分析" },
  { value: "code-generation", label: "代码生成" },
  { value: "code-review", label: "代码 Review" },
  { value: "test-generation", label: "测试生成" },
  { value: "fix-generation", label: "修复生成" },
  { value: "doc-generation", label: "文档生成" },
];

const PURPOSE_LABELS: Record<string, string> = {
  "requirement-analysis": "需求分析",
  "code-generation": "代码生成",
  "code-review": "代码 Review",
  "test-generation": "测试生成",
  "fix-generation": "修复生成",
  "doc-generation": "文档生成",
};

const PURPOSE_COLORS: Record<string, string> = {
  "requirement-analysis": "bg-blue-500/10 text-blue-400",
  "code-generation": "bg-green-500/10 text-green-400",
  "code-review": "bg-orange-500/10 text-orange-400",
  "test-generation": "bg-purple-500/10 text-purple-400",
  "fix-generation": "bg-red-500/10 text-red-400",
  "doc-generation": "bg-cyan-500/10 text-cyan-400",
};

interface PromptForm {
  name: string;
  purpose: string;
  systemPrompt: string;
  userTemplate: string;
  variables: string[];
  isDefault: boolean;
}

const EMPTY_FORM: PromptForm = {
  name: "",
  purpose: "requirement-analysis",
  systemPrompt: "",
  userTemplate: "",
  variables: [],
  isDefault: false,
};

export default function PromptsPage() {
  const [templates, setTemplates] = useState<PromptTemplate[]>([]);
  const [total, setTotal] = useState(0);
  const [page, setPage] = useState(1);
  const [purpose, setPurpose] = useState("");
  const [loading, setLoading] = useState(true);

  // Dialog state
  const [dialogOpen, setDialogOpen] = useState(false);
  const [editingTemplate, setEditingTemplate] = useState<PromptTemplate | null>(
    null
  );
  const [form, setForm] = useState<PromptForm>(EMPTY_FORM);
  const [variableInput, setVariableInput] = useState("");

  const pageSize = 20;
  const totalPages = Math.ceil(total / pageSize);

  const fetchTemplates = useCallback(async () => {
    setLoading(true);
    try {
      const result = await listPromptTemplates({
        purpose: purpose || undefined,
        page,
        pageSize,
      });
      setTemplates(result.items || []);
      setTotal(result.total);
    } catch (err) {
      console.error("Failed to fetch prompt templates:", err);
    } finally {
      setLoading(false);
    }
  }, [purpose, page]);

  useEffect(() => {
    fetchTemplates();
  }, [fetchTemplates]);

  const openCreate = () => {
    setEditingTemplate(null);
    setForm(EMPTY_FORM);
    setVariableInput("");
    setDialogOpen(true);
  };

  const openEdit = (tpl: PromptTemplate) => {
    setEditingTemplate(tpl);
    setForm({
      name: tpl.name,
      purpose: tpl.purpose,
      systemPrompt: tpl.systemPrompt,
      userTemplate: tpl.userTemplate,
      variables: [...tpl.variables],
      isDefault: tpl.isDefault,
    });
    setVariableInput("");
    setDialogOpen(true);
  };

  const handleSave = async () => {
    try {
      if (editingTemplate) {
        await updatePromptTemplate(editingTemplate.id, {
          name: form.name,
          purpose: form.purpose,
          systemPrompt: form.systemPrompt,
          userTemplate: form.userTemplate,
          variables: form.variables,
          isDefault: form.isDefault,
        });
      } else {
        await createPromptTemplate({
          name: form.name,
          purpose: form.purpose,
          systemPrompt: form.systemPrompt,
          userTemplate: form.userTemplate,
          variables: form.variables,
          isDefault: form.isDefault,
        });
      }
      setDialogOpen(false);
      fetchTemplates();
    } catch (err) {
      console.error("Failed to save prompt template:", err);
    }
  };

  const handleDelete = async (id: number) => {
    if (!confirm("确定要删除此 Prompt 模板吗？")) return;
    try {
      await deletePromptTemplate(id);
      fetchTemplates();
    } catch (err) {
      console.error("Failed to delete prompt template:", err);
    }
  };

  const addVariable = () => {
    const v = variableInput.trim();
    if (v && !form.variables.includes(v)) {
      setForm({ ...form, variables: [...form.variables, v] });
      setVariableInput("");
    }
  };

  const removeVariable = (variable: string) => {
    setForm({
      ...form,
      variables: form.variables.filter((v) => v !== variable),
    });
  };

  const handleVariableKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === "Enter") {
      e.preventDefault();
      addVariable();
    }
  };

  return (
    <div className="space-y-4">
      {/* Toolbar */}
      <div className="flex items-center justify-between gap-4">
        <div className="flex items-center gap-3">
          <Select
            value={purpose || "all"}
            onValueChange={(v) => {
              setPurpose(v === "all" ? "" : (v ?? ""));
              setPage(1);
            }}
          >
            <SelectTrigger className="w-[160px] bg-white/5 border-white/10 text-white">
              <SelectValue placeholder="全部用途" />
            </SelectTrigger>
            <SelectContent>
              {PURPOSES.map((p) => (
                <SelectItem key={p.value || "all"} value={p.value || "all"}>
                  {p.label}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        </div>
        <Button
          onClick={openCreate}
          className="bg-[#8B5CF6] hover:bg-[#7C3AED] text-white"
        >
          <Plus className="h-4 w-4 mr-2" />
          新建模板
        </Button>
      </div>

      {/* Table */}
      <div className="bg-white/[0.03] border border-white/10 rounded-lg overflow-hidden">
        <table className="w-full">
          <thead>
            <tr className="border-b border-white/10 text-white/50 text-sm">
              <th className="text-left px-4 py-3 font-medium">名称</th>
              <th className="text-left px-4 py-3 font-medium">用途</th>
              <th className="text-left px-4 py-3 font-medium">版本</th>
              <th className="text-left px-4 py-3 font-medium">默认</th>
              <th className="text-left px-4 py-3 font-medium">更新时间</th>
              <th className="text-right px-4 py-3 font-medium">操作</th>
            </tr>
          </thead>
          <tbody>
            {loading ? (
              <tr>
                <td colSpan={6} className="text-center py-12 text-white/30">
                  加载中...
                </td>
              </tr>
            ) : templates.length === 0 ? (
              <tr>
                <td colSpan={6} className="text-center py-12 text-white/30">
                  暂无 Prompt 模板，点击&ldquo;新建模板&rdquo;添加
                </td>
              </tr>
            ) : (
              templates.map((tpl) => (
                <tr
                  key={tpl.id}
                  className="border-b border-white/5 hover:bg-white/[0.02] transition-colors"
                >
                  <td className="px-4 py-3 text-white font-medium">
                    {tpl.name}
                  </td>
                  <td className="px-4 py-3">
                    <span
                      className={`inline-flex px-2 py-0.5 rounded text-xs font-medium ${
                        PURPOSE_COLORS[tpl.purpose] ||
                        "bg-white/10 text-white/70"
                      }`}
                    >
                      {PURPOSE_LABELS[tpl.purpose] || tpl.purpose}
                    </span>
                  </td>
                  <td className="px-4 py-3 text-white/50 text-sm">
                    v{tpl.version}
                  </td>
                  <td className="px-4 py-3">
                    <Star
                      className={`h-4 w-4 ${
                        tpl.isDefault
                          ? "fill-yellow-400 text-yellow-400"
                          : "text-white/20"
                      }`}
                    />
                  </td>
                  <td className="px-4 py-3 text-white/50 text-sm">
                    {new Date(tpl.updatedAt).toLocaleDateString("zh-CN")}
                  </td>
                  <td className="px-4 py-3 text-right">
                    <div className="flex items-center justify-end gap-1">
                      <Button
                        variant="ghost"
                        size="icon"
                        className="h-8 w-8 text-white/50 hover:text-white"
                        onClick={() => openEdit(tpl)}
                      >
                        <Edit2 className="h-4 w-4" />
                      </Button>
                      <Button
                        variant="ghost"
                        size="icon"
                        className="h-8 w-8 text-white/50 hover:text-red-400"
                        onClick={() => handleDelete(tpl.id)}
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
        <div className="flex items-center justify-between text-sm text-white/50">
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
        <DialogContent className="bg-[#0A0A12] border-white/10 text-white max-w-2xl max-h-[90vh] overflow-y-auto">
          <DialogHeader>
            <DialogTitle>
              {editingTemplate ? "编辑 Prompt 模板" : "新建 Prompt 模板"}
            </DialogTitle>
          </DialogHeader>
          <div className="space-y-4 py-4">
            {/* Name */}
            <div className="space-y-2">
              <Label>名称</Label>
              <Input
                value={form.name}
                onChange={(e) => setForm({ ...form, name: e.target.value })}
                placeholder="如：代码生成默认模板"
                className="bg-white/5 border-white/10"
              />
            </div>

            {/* Purpose */}
            <div className="space-y-2">
              <Label>用途</Label>
              <Select
                value={form.purpose}
                onValueChange={(v) => v && setForm({ ...form, purpose: v })}
              >
                <SelectTrigger className="bg-white/5 border-white/10">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  {PURPOSES.filter((p) => p.value).map((p) => (
                    <SelectItem key={p.value} value={p.value}>
                      {p.label}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>

            {/* System Prompt */}
            <div className="space-y-2">
              <Label>System Prompt</Label>
              <Textarea
                value={form.systemPrompt}
                onChange={(e) =>
                  setForm({ ...form, systemPrompt: e.target.value })
                }
                placeholder="输入系统提示词..."
                className="bg-[#0A0A12] border-white/10 font-mono text-sm min-h-[160px]"
              />
            </div>

            {/* User Template */}
            <div className="space-y-2">
              <Label>User Template</Label>
              <Textarea
                value={form.userTemplate}
                onChange={(e) =>
                  setForm({ ...form, userTemplate: e.target.value })
                }
                placeholder="输入用户模板，使用 {{variable}} 引用变量..."
                className="bg-[#0A0A12] border-white/10 font-mono text-sm min-h-[160px]"
              />
            </div>

            {/* Variables */}
            <div className="space-y-2">
              <Label>变量</Label>
              <div className="flex items-center gap-2">
                <Input
                  value={variableInput}
                  onChange={(e) => setVariableInput(e.target.value)}
                  onKeyDown={handleVariableKeyDown}
                  placeholder="输入变量名，回车添加"
                  className="bg-white/5 border-white/10 flex-1"
                />
                <Button
                  type="button"
                  variant="ghost"
                  onClick={addVariable}
                  className="text-[#8B5CF6] hover:text-[#7C3AED] shrink-0"
                  disabled={!variableInput.trim()}
                >
                  添加
                </Button>
              </div>
              {form.variables.length > 0 && (
                <div className="flex flex-wrap gap-2 mt-2">
                  {form.variables.map((v) => (
                    <span
                      key={v}
                      className="inline-flex items-center gap-1 px-2 py-0.5 rounded bg-[#8B5CF6]/10 text-[#8B5CF6] text-xs font-medium"
                    >
                      {v}
                      <button
                        type="button"
                        onClick={() => removeVariable(v)}
                        className="hover:text-red-400 transition-colors"
                      >
                        <X className="h-3 w-3" />
                      </button>
                    </span>
                  ))}
                </div>
              )}
            </div>

            {/* Is Default */}
            <div className="flex items-center justify-between rounded-lg border border-white/10 px-4 py-3">
              <div className="space-y-0.5">
                <Label className="text-sm font-medium">设为默认模板</Label>
                <p className="text-xs text-white/40">
                  同用途下仅一个默认模板，新设默认将替换旧默认
                </p>
              </div>
              <button
                type="button"
                role="switch"
                aria-checked={form.isDefault}
                onClick={() =>
                  setForm({ ...form, isDefault: !form.isDefault })
                }
                className={`relative inline-flex h-6 w-11 shrink-0 cursor-pointer rounded-full border-2 border-transparent transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[#8B5CF6] ${
                  form.isDefault ? "bg-[#8B5CF6]" : "bg-white/10"
                }`}
              >
                <span
                  className={`pointer-events-none inline-block h-5 w-5 rounded-full bg-white shadow-lg ring-0 transition-transform ${
                    form.isDefault ? "translate-x-5" : "translate-x-0"
                  }`}
                />
              </button>
            </div>
          </div>
          <DialogFooter>
            <Button
              variant="ghost"
              onClick={() => setDialogOpen(false)}
              className="text-white/50"
            >
              取消
            </Button>
            <Button
              onClick={handleSave}
              className="bg-[#8B5CF6] hover:bg-[#7C3AED] text-white"
              disabled={!form.name || !form.systemPrompt}
            >
              {editingTemplate ? "保存修改" : "创建"}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  );
}
