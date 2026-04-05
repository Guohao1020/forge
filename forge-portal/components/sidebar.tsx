"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import { ForgeLogo } from "./forge-logo";
import { FolderOpen, BookOpen, Settings, Users, KeyRound, BarChart3, Settings2 } from "lucide-react";

const navItems = [
  { href: "/projects", label: "项目大厅", icon: FolderOpen },
  { href: "/specs", label: "规范中心", icon: BookOpen },
];

const settingsItems = [
  { href: "/settings/dashboard", label: "仪表盘", icon: BarChart3 },
  { href: "/settings/users", label: "用户管理", icon: Users },
  { href: "/settings/platform", label: "平台配置", icon: Settings2 },
  { href: "/settings/account", label: "账户设置", icon: KeyRound },
];

export function Sidebar() {
  const pathname = usePathname();

  return (
    <aside className="w-60 h-screen flex flex-col border-r border-[var(--border)] bg-[var(--card)]">
      <div className="h-14 flex items-center px-5 border-b border-[var(--border)]">
        <ForgeLogo />
      </div>

      <nav className="flex-1 p-3 space-y-1">
        {navItems.map((item) => {
          const active = pathname === item.href || pathname.startsWith(item.href + "/");
          return (
            <Link
              key={item.href}
              href={item.href}
              className={`flex items-center gap-3 px-3 py-2 rounded-lg text-sm transition-colors ${
                active
                  ? "text-[var(--foreground)] bg-[rgba(139,92,246,0.1)]"
                  : "text-[var(--muted-foreground)] hover:text-[var(--foreground)]"
              }`}
            >
              <item.icon size={18} />
              {item.label}
            </Link>
          );
        })}
      </nav>

      <div className="p-3 border-t border-[var(--border)]">
        <div className="flex items-center gap-2 px-3 py-1.5 text-xs text-[var(--muted-foreground)] uppercase tracking-wider">
          <Settings size={14} />
          平台设置
        </div>
        {settingsItems.map((item) => {
          const active = pathname === item.href;
          return (
            <Link
              key={item.href}
              href={item.href}
              className={`flex items-center gap-3 px-3 py-1.5 rounded-lg text-xs transition-colors ${
                active
                  ? "text-[var(--foreground)] bg-[rgba(139,92,246,0.1)]"
                  : "text-[var(--muted-foreground)] hover:text-[var(--foreground)]"
              }`}
            >
              <item.icon size={14} />
              {item.label}
            </Link>
          );
        })}
      </div>
    </aside>
  );
}
