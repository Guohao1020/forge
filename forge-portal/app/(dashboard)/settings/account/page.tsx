"use client";

import { useState } from "react";
import { KeyRound } from "lucide-react";
import { api } from "@/lib/api";

export default function AccountSettingsPage() {
  const [oldPassword, setOldPassword] = useState("");
  const [newPassword, setNewPassword] = useState("");
  const [confirmPassword, setConfirmPassword] = useState("");
  const [saving, setSaving] = useState(false);
  const [message, setMessage] = useState<{ type: "success" | "error"; text: string } | null>(null);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setMessage(null);

    if (newPassword.length < 6) {
      setMessage({ type: "error", text: "新密码至少 6 位" });
      return;
    }
    if (newPassword !== confirmPassword) {
      setMessage({ type: "error", text: "两次输入的新密码不一致" });
      return;
    }

    setSaving(true);
    try {
      await api.put("/auth/password", { oldPassword, newPassword });
      setMessage({ type: "success", text: "密码修改成功" });
      setOldPassword("");
      setNewPassword("");
      setConfirmPassword("");
    } catch (err: unknown) {
      setMessage({ type: "error", text: err instanceof Error ? err.message : "修改失败" });
    } finally {
      setSaving(false);
    }
  };

  return (
    <div className="max-w-lg space-y-6">
      <div>
        <h1 className="text-xl font-semibold text-foreground">账户设置</h1>
        <p className="text-sm text-muted-foreground mt-1">管理你的账户密码</p>
      </div>

      <form onSubmit={handleSubmit} className="bg-surface-1 border border-border rounded-lg p-5 space-y-4">
        <div className="flex items-center gap-2 text-sm font-medium text-foreground mb-2">
          <KeyRound size={16} className="text-primary" />
          修改密码
        </div>

        <div>
          <label className="block text-xs text-muted-foreground mb-1">当前密码</label>
          <input
            type="password"
            value={oldPassword}
            onChange={(e) => setOldPassword(e.target.value)}
            placeholder="输入当前密码"
            className="w-full bg-muted/50 border border-border rounded-lg px-3 py-2 text-sm text-foreground placeholder:text-muted-foreground/60 focus:outline-none focus:border-primary/50"
            required
          />
        </div>

        <div>
          <label className="block text-xs text-muted-foreground mb-1">新密码</label>
          <input
            type="password"
            value={newPassword}
            onChange={(e) => setNewPassword(e.target.value)}
            placeholder="至少 6 位"
            className="w-full bg-muted/50 border border-border rounded-lg px-3 py-2 text-sm text-foreground placeholder:text-muted-foreground/60 focus:outline-none focus:border-primary/50"
            required
            minLength={6}
          />
        </div>

        <div>
          <label className="block text-xs text-muted-foreground mb-1">确认新密码</label>
          <input
            type="password"
            value={confirmPassword}
            onChange={(e) => setConfirmPassword(e.target.value)}
            placeholder="再次输入新密码"
            className="w-full bg-muted/50 border border-border rounded-lg px-3 py-2 text-sm text-foreground placeholder:text-muted-foreground/60 focus:outline-none focus:border-primary/50"
            required
            minLength={6}
          />
        </div>

        {message && (
          <div className={`text-sm px-3 py-2 rounded-lg ${
            message.type === "success"
              ? "bg-green-500/10 text-green-400 border border-green-500/20"
              : "bg-red-500/10 text-red-400 border border-red-500/20"
          }`}>
            {message.text}
          </div>
        )}

        <div className="flex justify-end">
          <button
            type="submit"
            disabled={saving || !oldPassword || !newPassword || !confirmPassword}
            className="px-4 py-2 bg-primary text-primary-foreground rounded-lg text-sm hover:bg-primary/90 disabled:opacity-50 transition-colors"
          >
            {saving ? "修改中..." : "修改密码"}
          </button>
        </div>
      </form>
    </div>
  );
}
