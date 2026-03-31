"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import { ForgeLogo } from "./forge-logo";
import { FolderOpen } from "lucide-react";

const navItems = [
  { href: "/projects", label: "项目大厅", icon: FolderOpen },
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
          const active = pathname === item.href;
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
    </aside>
  );
}
