"use client";

import { useEffect, useState } from "react";
import { Plus, Shield, UserCircle } from "lucide-react";
import { listUsers, createUser, updateUserRole, UserItem, ROLE_CONFIG } from "@/lib/users";

const ALL_ROLES = ["PLATFORM_ADMIN", "ORG_ADMIN", "PROJECT_ADMIN", "DEVELOPER", "VIEWER"];

export default function UsersPage() {
  const [users, setUsers] = useState<UserItem[]>([]);
  const [loading, setLoading] = useState(true);
  const [showCreate, setShowCreate] = useState(false);
  const [newUsername, setNewUsername] = useState("");
  const [newPassword, setNewPassword] = useState("");
  const [newDisplayName, setNewDisplayName] = useState("");
  const [newRole, setNewRole] = useState("DEVELOPER");
  const [creating, setCreating] = useState(false);

  const fetchUsers = async () => {
    try {
      const res = await listUsers();
      setUsers(res.users || []);
    } catch {
      // ignore
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => { fetchUsers(); }, []);

  const handleCreate = async () => {
    if (!newUsername.trim() || !newPassword.trim()) return;
    setCreating(true);
    try {
      await createUser({
        username: newUsername.trim(),
        password: newPassword,
        displayName: newDisplayName.trim() || undefined,
        role: newRole,
      });
      setShowCreate(false);
      setNewUsername("");
      setNewPassword("");
      setNewDisplayName("");
      setNewRole("DEVELOPER");
      await fetchUsers();
    } catch (e: unknown) {
      alert(e instanceof Error ? e.message : "创建失败");
    } finally {
      setCreating(false);
    }
  };

  const handleRoleChange = async (userId: number, role: string) => {
    try {
      await updateUserRole(userId, role);
      await fetchUsers();
    } catch (e: unknown) {
      alert(e instanceof Error ? e.message : "角色更新失败");
    }
  };

  if (loading) {
    return (
      <div className="space-y-3">
        {[1, 2, 3].map((i) => (
          <div key={i} className="h-16 rounded-lg bg-muted/50 animate-pulse" />
        ))}
      </div>
    );
  }

  return (
    <div className="max-w-3xl space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-xl font-semibold text-foreground">用户管理</h1>
          <p className="text-sm text-muted-foreground mt-1">管理平台用户和角色权限</p>
        </div>
        <button
          onClick={() => setShowCreate(true)}
          className="flex items-center gap-2 px-4 py-2 bg-primary text-primary-foreground rounded-lg text-sm hover:bg-primary/90 transition-colors"
        >
          <Plus size={16} />
          添加用户
        </button>
      </div>

      {showCreate && (
        <div className="bg-surface-1 border border-border rounded-lg p-4 space-y-3">
          <div className="grid grid-cols-2 gap-3">
            <div>
              <label className="block text-xs text-muted-foreground mb-1">用户名</label>
              <input
                type="text"
                value={newUsername}
                onChange={(e) => setNewUsername(e.target.value)}
                placeholder="username"
                className="w-full bg-muted/50 border border-border rounded-lg px-3 py-2 text-sm text-foreground placeholder:text-muted-foreground/60 focus:outline-none focus:border-primary/50"
                autoFocus
              />
            </div>
            <div>
              <label className="block text-xs text-muted-foreground mb-1">密码</label>
              <input
                type="password"
                value={newPassword}
                onChange={(e) => setNewPassword(e.target.value)}
                placeholder="至少6位"
                className="w-full bg-muted/50 border border-border rounded-lg px-3 py-2 text-sm text-foreground placeholder:text-muted-foreground/60 focus:outline-none focus:border-primary/50"
              />
            </div>
          </div>
          <div className="grid grid-cols-2 gap-3">
            <div>
              <label className="block text-xs text-muted-foreground mb-1">显示名称</label>
              <input
                type="text"
                value={newDisplayName}
                onChange={(e) => setNewDisplayName(e.target.value)}
                placeholder="可选"
                className="w-full bg-muted/50 border border-border rounded-lg px-3 py-2 text-sm text-foreground placeholder:text-muted-foreground/60 focus:outline-none focus:border-primary/50"
              />
            </div>
            <div>
              <label className="block text-xs text-muted-foreground mb-1">角色</label>
              <select
                value={newRole}
                onChange={(e) => setNewRole(e.target.value)}
                className="w-full bg-muted/50 border border-border rounded-lg px-3 py-2 text-sm text-foreground focus:outline-none focus:border-primary/50"
              >
                {ALL_ROLES.map((r) => (
                  <option key={r} value={r}>
                    {ROLE_CONFIG[r]?.label || r}
                  </option>
                ))}
              </select>
            </div>
          </div>
          <div className="flex gap-2 justify-end">
            <button onClick={() => setShowCreate(false)} className="px-3 py-1.5 text-sm text-muted-foreground hover:text-foreground">取消</button>
            <button
              onClick={handleCreate}
              disabled={creating || !newUsername.trim() || !newPassword.trim()}
              className="px-4 py-1.5 bg-primary text-primary-foreground rounded-lg text-sm hover:bg-primary/90 disabled:opacity-50"
            >
              {creating ? "创建中..." : "创建用户"}
            </button>
          </div>
        </div>
      )}

      <div className="space-y-2">
        {users.map((user) => (
          <div key={user.id} className="flex items-center justify-between bg-surface-1 border border-border rounded-lg px-4 py-3">
            <div className="flex items-center gap-3">
              <UserCircle size={32} className="text-muted-foreground" />
              <div>
                <p className="text-sm font-medium text-foreground">{user.displayName || user.username}</p>
                <p className="text-xs text-muted-foreground">@{user.username}</p>
              </div>
            </div>
            <div className="flex items-center gap-3">
              {user.roles.map((role) => {
                const config = ROLE_CONFIG[role] || { label: role, color: "text-gray-400 bg-gray-500/10 border-gray-500/20" };
                return (
                  <span key={role} className={`flex items-center gap-1 px-2 py-0.5 rounded text-xs border ${config.color}`}>
                    <Shield size={10} />
                    {config.label}
                  </span>
                );
              })}
              <select
                value={user.roles[0] || "VIEWER"}
                onChange={(e) => handleRoleChange(user.id, e.target.value)}
                className="bg-muted/50 border border-border rounded px-2 py-1 text-xs text-muted-foreground focus:outline-none"
              >
                {ALL_ROLES.map((r) => (
                  <option key={r} value={r}>{ROLE_CONFIG[r]?.label || r}</option>
                ))}
              </select>
            </div>
          </div>
        ))}
      </div>
    </div>
  );
}
