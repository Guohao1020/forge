"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import {
  ArrowLeft,
  Layers,
  Code2,
  GitCommit,
  FlaskConical,
  Rocket,
  Package,
  Brain,
  Settings,
  Tags,
  Shield,
  Terminal,
  BookOpen,
} from "lucide-react";
import { ForgeLogo } from "./forge-logo";

interface ProjectSidebarProps {
  projectId: string;
  projectName: string;
}

export function ProjectSidebar({ projectId, projectName }: ProjectSidebarProps) {
  const pathname = usePathname();
  const base = `/projects/${projectId}`;

  const navItems = [
    { href: `${base}/agent`, label: "Agent", icon: Terminal, exact: true },
    { href: `${base}/code`, label: "代码", icon: Code2 },
    { href: `${base}/changes`, label: "变更", icon: GitCommit },
    { href: `${base}/tests`, label: "测试", icon: FlaskConical },
    { href: `${base}/artifacts`, label: "制品", icon: Package },
    { href: `${base}/versions`, label: "版本", icon: Tags },
    { href: `${base}/deploy`, label: "部署", icon: Rocket },
    { href: `${base}/profile`, label: "画像", icon: Brain },
    { href: `${base}/quality`, label: "质量", icon: Shield },
    { href: `${base}/skills`, label: "技能", icon: BookOpen },
    { href: `${base}/settings`, label: "设置", icon: Settings },
  ];

  return (
    <aside className="w-60 h-screen flex flex-col border-r border-border bg-card shrink-0">
      <div className="h-14 flex items-center px-5 border-b border-border">
        <ForgeLogo />
      </div>

      {/* Project name + back link */}
      <div className="px-3 pt-3 pb-2">
        <Link
          href="/projects"
          className="flex items-center gap-1.5 text-xs text-muted-foreground hover:text-foreground transition-colors mb-3"
        >
          <ArrowLeft className="h-3 w-3" />
          项目大厅
        </Link>
        <div className="px-3 py-2 rounded-lg bg-primary/10">
          <p className="text-xs text-muted-foreground mb-0.5">当前项目</p>
          <p className="text-sm font-medium text-foreground truncate">{projectName}</p>
        </div>
      </div>

      <nav className="flex-1 p-3 space-y-1">
        {navItems.map((item) => {
          const active = item.exact
            ? pathname === item.href
            : pathname.startsWith(item.href);
          return (
            <Link
              key={item.href}
              href={item.href}
              className={`flex items-center gap-3 px-3 py-2 rounded-lg text-sm transition-colors ${
                active
                  ? "text-foreground bg-primary/10"
                  : "text-muted-foreground hover:text-foreground"
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
