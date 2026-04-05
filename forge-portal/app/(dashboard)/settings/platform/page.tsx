"use client";

import { useEffect, useState } from "react";
import { Settings2, Save } from "lucide-react";
import { api } from "@/lib/api";

interface Setting {
  key: string;
  value: string;
  category: string;
  updatedAt?: string;
}

const CATEGORY_LABELS: Record<string, string> = {
  ai: "AI 模型配置",
  deploy: "部署配置",
  notification: "通知配置",
  general: "通用配置",
};

const SETTING_LABELS: Record<string, { label: string; description: string; type?: "toggle" | "text" }> = {
  "ai.default_model": { label: "默认模型", description: "AI 任务使用的默认 LLM 模型" },
  "ai.fallback_chain": { label: "降级链", description: "模型不可用时的降级顺序（逗号分隔）" },
  "ai.max_tokens": { label: "最大 Token", description: "单次 LLM 调用的最大 Token 数" },
  "ai.temperature": { label: "Temperature", description: "LLM 生成温度（0-1，越低越确定）" },
  "deploy.auto_merge": { label: "自动合并", description: "低风险任务完成后自动合并 PR", type: "toggle" },
  "deploy.require_review": { label: "需要审批", description: "部署前需要人工审批", type: "toggle" },
  "notify.slack_webhook": { label: "Slack Webhook", description: "任务通知的 Slack Webhook URL" },
  "notify.on_completion": { label: "完成通知", description: "任务完成时发送通知", type: "toggle" },
  "general.language": { label: "界面语言", description: "平台界面语言" },
  "general.timezone": { label: "时区", description: "平台默认时区" },
  "entropy.default_schedule": { label: "默认扫描频率", description: "新项目的默认质量扫描频率" },
};

export default function PlatformSettingsPage() {
  const [settings, setSettings] = useState<Setting[]>([]);
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [changes, setChanges] = useState<Record<string, string>>({});
  const [message, setMessage] = useState("");

  useEffect(() => {
    api.get<{ settings: Setting[] }>("/settings")
      .then((res) => setSettings(res.settings))
      .catch(() => {})
      .finally(() => setLoading(false));
  }, []);

  const handleChange = (key: string, value: string) => {
    setChanges((prev) => ({ ...prev, [key]: value }));
  };

  const getValue = (key: string) => {
    if (key in changes) return changes[key];
    const s = settings.find((s) => s.key === key);
    return s?.value || "";
  };

  const handleSave = async () => {
    if (Object.keys(changes).length === 0) return;
    setSaving(true);
    setMessage("");
    try {
      await api.put("/settings", changes);
      setMessage("保存成功");
      setChanges({});
      // Refresh
      const res = await api.get<{ settings: Setting[] }>("/settings");
      setSettings(res.settings);
      setTimeout(() => setMessage(""), 3000);
    } catch (e: unknown) {
      setMessage(e instanceof Error ? e.message : "保存失败");
    } finally {
      setSaving(false);
    }
  };

  if (loading) {
    return (
      <div className="space-y-4 max-w-2xl">
        {[1, 2, 3].map((i) => (
          <div key={i} className="h-24 rounded-xl bg-white/5 animate-pulse" />
        ))}
      </div>
    );
  }

  // Group settings by category
  const grouped: Record<string, Setting[]> = {};
  for (const s of settings) {
    if (!grouped[s.category]) grouped[s.category] = [];
    grouped[s.category].push(s);
  }

  return (
    <div className="max-w-2xl space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-xl font-semibold text-foreground flex items-center gap-2">
            <Settings2 className="h-5 w-5 text-primary" />
            平台配置
          </h1>
          <p className="text-sm text-muted-foreground mt-1">管理 AI 模型、部署策略、通知等全局设置</p>
        </div>
        <button
          onClick={handleSave}
          disabled={saving || Object.keys(changes).length === 0}
          className="flex items-center gap-2 px-4 py-2 bg-primary text-white rounded-lg text-sm hover:bg-primary/90 disabled:opacity-50 transition-colors"
        >
          <Save size={14} />
          {saving ? "保存中..." : `保存 (${Object.keys(changes).length})`}
        </button>
      </div>

      {message && (
        <div className={`text-sm px-4 py-2 rounded-lg ${
          message === "保存成功"
            ? "bg-green-500/10 text-green-400 border border-green-500/20"
            : "bg-red-500/10 text-red-400 border border-red-500/20"
        }`}>
          {message}
        </div>
      )}

      {Object.entries(grouped).map(([category, items]) => (
        <div key={category} className="rounded-xl border border-border bg-card p-5 space-y-4">
          <h2 className="text-sm font-medium text-foreground">
            {CATEGORY_LABELS[category] || category}
          </h2>
          <div className="space-y-3">
            {items.map((s) => {
              const meta = SETTING_LABELS[s.key];
              const isToggle = meta?.type === "toggle";
              const currentValue = getValue(s.key);

              return (
                <div key={s.key} className="flex items-center justify-between gap-4">
                  <div className="flex-1 min-w-0">
                    <p className="text-sm text-foreground">{meta?.label || s.key}</p>
                    <p className="text-xs text-muted-foreground">{meta?.description || ""}</p>
                  </div>
                  {isToggle ? (
                    <button
                      onClick={() => handleChange(s.key, currentValue === "true" ? "false" : "true")}
                      className={`relative w-10 h-5 rounded-full transition-colors shrink-0 ${
                        currentValue === "true" ? "bg-primary" : "bg-white/20"
                      }`}
                    >
                      <span className={`absolute top-0.5 w-4 h-4 rounded-full bg-white transition-transform ${
                        currentValue === "true" ? "left-5" : "left-0.5"
                      }`} />
                    </button>
                  ) : (
                    <input
                      type="text"
                      value={currentValue}
                      onChange={(e) => handleChange(s.key, e.target.value)}
                      className="w-56 bg-white/5 border border-white/10 rounded-lg px-3 py-1.5 text-sm text-foreground focus:outline-none focus:border-primary/50 shrink-0"
                    />
                  )}
                </div>
              );
            })}
          </div>
        </div>
      ))}
    </div>
  );
}
