"use client";

import { useState } from "react";
import { useAuth } from "@/lib/auth";
import { AuroraBackground } from "@/components/aurora-background";
import { ForgeLogo } from "@/components/forge-logo";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { ApiError } from "@/lib/api";

export default function LoginPage() {
  const { login } = useAuth();
  const [username, setUsername] = useState("");
  const [password, setPassword] = useState("");
  const [error, setError] = useState("");
  const [loading, setLoading] = useState(false);
  const [shake, setShake] = useState(false);

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    setError("");
    setLoading(true);

    try {
      await login(username, password);
    } catch (err) {
      const message = err instanceof ApiError ? err.message : "登录失败，请重试";
      setError(message);
      setShake(true);
      setTimeout(() => setShake(false), 500);
    } finally {
      setLoading(false);
    }
  }

  return (
    <div className="min-h-screen flex items-center justify-center">
      <AuroraBackground />

      <div
        className={`w-[480px] p-8 rounded-2xl border backdrop-blur-xl transition-transform ${
          shake ? "animate-shake" : ""
        }`}
        style={{
          background: "rgba(15, 15, 26, 0.8)",
          borderColor: "rgba(255, 255, 255, 0.08)",
          borderTopColor: "rgba(255, 255, 255, 0.12)",
        }}
      >
        <div className="text-center mb-8">
          <ForgeLogo className="mb-2" />
          <p className="text-sm text-[var(--muted-foreground)]">
            Harness Engineering Platform
          </p>
        </div>

        <form onSubmit={handleSubmit} className="space-y-5">
          <div className="space-y-2">
            <Label htmlFor="username" className="text-[var(--muted-foreground)]">
              用户名
            </Label>
            <Input
              id="username"
              value={username}
              onChange={(e) => setUsername(e.target.value)}
              placeholder="输入用户名"
              autoComplete="username"
              className="h-11 bg-[var(--input)] border-[var(--border)] text-[var(--foreground)] transition-all duration-150 focus:shadow-[0_0_0_2px_rgba(139,92,246,0.2)]"
              style={error ? { borderColor: "var(--destructive)" } : {}}
            />
          </div>

          <div className="space-y-2">
            <Label htmlFor="password" className="text-[var(--muted-foreground)]">
              密码
            </Label>
            <Input
              id="password"
              type="password"
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              placeholder="输入密码"
              autoComplete="current-password"
              className="h-11 bg-[var(--input)] border-[var(--border)] text-[var(--foreground)] transition-all duration-150 focus:shadow-[0_0_0_2px_rgba(139,92,246,0.2)]"
              style={error ? { borderColor: "var(--destructive)" } : {}}
            />
          </div>

          {error && (
            <p className="text-sm text-[var(--destructive)]">
              {error}
            </p>
          )}

          <Button
            type="submit"
            disabled={loading || !username || !password}
            className="w-full h-11 text-base font-medium rounded-lg transition-all active:scale-[0.97] bg-[var(--primary)] hover:bg-[#7C3AED] text-white"
            style={{
              boxShadow: "0 0 20px rgba(139, 92, 246, 0.3)",
            }}
          >
            {loading ? "登录中..." : "登录"}
          </Button>
        </form>

        <p className="text-center mt-6 text-xs text-[var(--muted-foreground)]">
          Forge v0.1.0
        </p>
      </div>
    </div>
  );
}
