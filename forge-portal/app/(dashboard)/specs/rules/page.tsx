"use client";

import { useState, useEffect, useCallback } from "react";
import {
  Plus,
  Edit2,
  ChevronLeft,
  ChevronRight,
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
import { Switch } from "@/components/ui/switch";
import {
  ReviewRule,
  listReviewRules,
  createReviewRule,
  updateReviewRule,
  toggleReviewRule,
} from "@/lib/specs";

const CATEGORIES = [
  { value: "", label: "全部分类" },
  { value: "CODING", label: "编码" },
  { value: "SECURITY", label: "安全" },
  { value: "PERFORMANCE", label: "性能" },
  { value: "DATABASE", label: "数据库" },
  { value: "API_COMPAT", label: "API 兼容" },
  { value: "CUSTOM", label: "自定义" },
];

const SEVERITIES = [
  { value: "", label: "全部严重度" },
  { value: "ERROR", label: "ERROR" },
  { value: "WARNING", label: "WARNING" },
  { value: "INFO", label: "INFO" },
];

const SCOPES = [
  { value: "COMPANY", label: "公司级" },
  { value: "TEAM", label: "团队级" },
  { value: "PROJECT", label: "项目级" },
];

const RULE_TYPES = [
  { value: "PATTERN", label: "PATTERN" },
  { value: "AST", label: "AST" },
  { value: "AI_CHECK", label: "AI_CHECK" },
];

const CATEGORY_LABELS: Record<string, string> = {
  CODING: "编码",
  SECURITY: "安全",
  PERFORMANCE: "性能",
  DATABASE: "数据库",
  API_COMPAT: "API 兼容",
  CUSTOM: "自定义",
};

const CATEGORY_COLORS: Record<string, string> = {
  CODING: "bg-blue-500/10 text-blue-400",
  SECURITY: "bg-yellow-500/10 text-yellow-400",
  PERFORMANCE: "bg-green-500/10 text-green-400",
  DATABASE: "bg-orange-500/10 text-orange-400",
  API_COMPAT: "bg-purple-500/10 text-purple-400",
  CUSTOM: "bg-cyan-500/10 text-cyan-400",
};

const SEVERITY_COLORS: Record<string, string> = {
  ERROR: "bg-red-500/10 text-red-400",
  WARNING: "bg-yellow-500/10 text-yellow-400",
  INFO: "bg-blue-500/10 text-blue-400",
};

interface RuleForm {
  name: string;
  category: string;
  scope: string;
  scopeId: number;
  ruleType: string;
  definition: string;
  severity: string;
  autoFix: boolean;
  fixTemplate: string;
}

const defaultForm: RuleForm = {
  name: "",
  category: "CODING",
  scope: "COMPANY",
  scopeId: 0,
  ruleType: "PATTERN",
  definition: "{}",
  severity: "WARNING",
  autoFix: false,
  fixTemplate: "",
};

export default function ReviewRulesPage() {
  const [rules, setRules] = useState<ReviewRule[]>([]);
  const [total, setTotal] = useState(0);
  const [page, setPage] = useState(1);
  const [category, setCategory] = useState("");
  const [severity, setSeverity] = useState("");
  const [loading, setLoading] = useState(true);

  // Dialog state
  const [dialogOpen, setDialogOpen] = useState(false);
  const [editingRule, setEditingRule] = useState<ReviewRule | null>(null);
  const [form, setForm] = useState<RuleForm>({ ...defaultForm });

  const pageSize = 20;
  const totalPages = Math.ceil(total / pageSize);

  const fetchRules = useCallback(async () => {
    setLoading(true);
    try {
      const result = await listReviewRules({
        category: category || undefined,
        severity: severity || undefined,
        page,
        pageSize,
      });
      setRules(result.items || []);
      setTotal(result.total);
    } catch (err) {
      console.error("Failed to fetch review rules:", err);
    } finally {
      setLoading(false);
    }
  }, [category, severity, page]);

  useEffect(() => {
    fetchRules();
  }, [fetchRules]);

  const openCreate = () => {
    setEditingRule(null);
    setForm({ ...defaultForm });
    setDialogOpen(true);
  };

  const openEdit = (rule: ReviewRule) => {
    setEditingRule(rule);
    setForm({
      name: rule.name,
      category: rule.category,
      scope: rule.scope,
      scopeId: rule.scopeId,
      ruleType: rule.ruleType,
      definition: JSON.stringify(rule.definition, null, 2),
      severity: rule.severity,
      autoFix: rule.autoFix,
      fixTemplate: rule.fixTemplate || "",
    });
    setDialogOpen(true);
  };

  const handleSave = async () => {
    try {
      let definition: Record<string, unknown>;
      try {
        definition = JSON.parse(form.definition);
      } catch {
        alert("Definition 必须是有效的 JSON 格式");
        return;
      }

      const payload = {
        name: form.name,
        category: form.category,
        ruleType: form.ruleType,
        definition,
        severity: form.severity,
        autoFix: form.autoFix,
        fixTemplate: form.autoFix ? form.fixTemplate : undefined,
      };

      if (editingRule) {
        await updateReviewRule(editingRule.id, payload);
      } else {
        await createReviewRule({
          ...payload,
          scope: form.scope,
          scopeId: form.scopeId,
        });
      }
      setDialogOpen(false);
      fetchRules();
    } catch (err) {
      console.error("Failed to save review rule:", err);
    }
  };

  const handleToggle = async (id: number) => {
    try {
      await toggleReviewRule(id);
      fetchRules();
    } catch (err) {
      console.error("Failed to toggle review rule:", err);
    }
  };

  return (
    <div className="space-y-4">
      {/* Toolbar */}
      <div className="flex items-center justify-between gap-4">
        <div className="flex items-center gap-3">
          <Select
            value={category || "all"}
            onValueChange={(v) => {
              setCategory(!v || v === "all" ? "" : v);
              setPage(1);
            }}
          >
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
          <Select
            value={severity || "all"}
            onValueChange={(v) => {
              setSeverity(!v || v === "all" ? "" : v);
              setPage(1);
            }}
          >
            <SelectTrigger className="w-[160px] bg-muted border-border">
              <SelectValue placeholder="全部严重度" />
            </SelectTrigger>
            <SelectContent>
              {SEVERITIES.map((s) => (
                <SelectItem key={s.value || "all"} value={s.value || "all"}>
                  {s.label}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        </div>
        <Button
          onClick={openCreate}
          className="bg-accent hover:bg-accent/90 text-accent-foreground"
        >
          <Plus className="h-4 w-4 mr-2" />
          新建规则
        </Button>
      </div>

      {/* Table */}
      <div className="bg-muted/30 border border-border rounded-lg overflow-hidden">
        <table className="w-full">
          <thead>
            <tr className="border-b border-border text-muted-foreground text-sm">
              <th className="text-left px-4 py-3 font-medium">名称</th>
              <th className="text-left px-4 py-3 font-medium">分类</th>
              <th className="text-left px-4 py-3 font-medium">严重度</th>
              <th className="text-left px-4 py-3 font-medium">规则类型</th>
              <th className="text-left px-4 py-3 font-medium">启用</th>
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
            ) : rules.length === 0 ? (
              <tr>
                <td colSpan={6} className="text-center py-12 text-muted-foreground">
                  暂无审查规则，点击&ldquo;新建规则&rdquo;添加
                </td>
              </tr>
            ) : (
              rules.map((rule) => (
                <tr
                  key={rule.id}
                  className="border-b border-border hover:bg-muted/20 transition-colors"
                >
                  <td className="px-4 py-3 text-foreground font-medium">
                    {rule.name}
                  </td>
                  <td className="px-4 py-3">
                    <span
                      className={`inline-flex px-2 py-0.5 rounded text-xs font-medium ${
                        CATEGORY_COLORS[rule.category] ||
                        "bg-muted text-muted-foreground"
                      }`}
                    >
                      {CATEGORY_LABELS[rule.category] || rule.category}
                    </span>
                  </td>
                  <td className="px-4 py-3">
                    <span
                      className={`inline-flex px-2 py-0.5 rounded text-xs font-medium ${
                        SEVERITY_COLORS[rule.severity] ||
                        "bg-muted text-muted-foreground"
                      }`}
                    >
                      {rule.severity}
                    </span>
                  </td>
                  <td className="px-4 py-3 text-muted-foreground text-sm">
                    {rule.ruleType}
                  </td>
                  <td className="px-4 py-3">
                    <Switch
                      checked={rule.enabled}
                      onCheckedChange={() => handleToggle(rule.id)}
                    />
                  </td>
                  <td className="px-4 py-3 text-right">
                    <Button
                      variant="ghost"
                      size="icon"
                      className="h-8 w-8 text-muted-foreground hover:text-foreground"
                      onClick={() => openEdit(rule)}
                    >
                      <Edit2 className="h-4 w-4" />
                    </Button>
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
        <DialogContent className="bg-background border-border max-w-2xl">
          <DialogHeader>
            <DialogTitle>
              {editingRule ? "编辑审查规则" : "新建审查规则"}
            </DialogTitle>
          </DialogHeader>
          <div className="space-y-4 py-4">
            <div className="space-y-2">
              <Label>名称</Label>
              <Input
                value={form.name}
                onChange={(e) => setForm({ ...form, name: e.target.value })}
                placeholder="如：禁止使用 SELECT *"
                className="bg-muted border-border"
              />
            </div>

            {!editingRule && (
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

            <div className="grid grid-cols-2 gap-4">
              <div className="space-y-2">
                <Label>规则类型</Label>
                <Select
                  value={form.ruleType}
                  onValueChange={(v) => v && setForm({ ...form, ruleType: v })}
                >
                  <SelectTrigger className="bg-muted border-border">
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    {RULE_TYPES.map((rt) => (
                      <SelectItem key={rt.value} value={rt.value}>
                        {rt.label}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </div>
              <div className="space-y-2">
                <Label>严重度</Label>
                <Select
                  value={form.severity}
                  onValueChange={(v) => v && setForm({ ...form, severity: v })}
                >
                  <SelectTrigger className="bg-muted border-border">
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    {SEVERITIES.filter((s) => s.value).map((s) => (
                      <SelectItem key={s.value} value={s.value}>
                        {s.label}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </div>
            </div>

            <div className="space-y-2">
              <Label>规则定义（JSON）</Label>
              <Textarea
                value={form.definition}
                onChange={(e) =>
                  setForm({ ...form, definition: e.target.value })
                }
                placeholder='{"pattern": "SELECT \\\\*", "message": "禁止使用 SELECT *"}'
                className="bg-[#FAFAFA] border-border font-mono text-sm min-h-[160px]"
              />
            </div>

            <div className="flex items-center justify-between">
              <Label>自动修复</Label>
              <Switch
                checked={form.autoFix}
                onCheckedChange={(checked) =>
                  setForm({ ...form, autoFix: checked })
                }
              />
            </div>

            {form.autoFix && (
              <div className="space-y-2">
                <Label>修复模板</Label>
                <Textarea
                  value={form.fixTemplate}
                  onChange={(e) =>
                    setForm({ ...form, fixTemplate: e.target.value })
                  }
                  placeholder="输入自动修复模板..."
                  className="bg-[#FAFAFA] border-border font-mono text-sm min-h-[120px]"
                />
              </div>
            )}
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
              disabled={!form.name || !form.definition}
            >
              {editingRule ? "保存修改" : "创建"}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  );
}
