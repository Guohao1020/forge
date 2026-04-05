"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import { ChevronRight, Home } from "lucide-react";

const PATH_LABELS: Record<string, string> = {
  projects: "项目大厅",
  specs: "规范中心",
  settings: "设置",
  code: "代码",
  tasks: "任务",
  versions: "版本",
  deploy: "部署",
  quality: "质量",
  profile: "画像",
  artifacts: "制品",
  tests: "测试",
  changes: "变更",
  webhooks: "Webhooks",
  users: "用户管理",
  account: "账户设置",
  dashboard: "仪表盘",
  platform: "平台配置",
  standards: "编码规范",
  prompts: "Prompt 模板",
  rules: "审查规则",
  scaffolds: "脚手架",
  new: "新建",
};

export function Breadcrumb() {
  const pathname = usePathname();
  const segments = pathname.split("/").filter(Boolean);

  if (segments.length <= 1) return null;

  const crumbs: { label: string; href: string }[] = [];
  let currentPath = "";

  for (let i = 0; i < segments.length; i++) {
    const segment = segments[i];
    currentPath += "/" + segment;

    // Skip numeric IDs (project IDs, task IDs, etc.)
    if (/^\d+$/.test(segment)) {
      continue;
    }

    const label = PATH_LABELS[segment] || segment;
    crumbs.push({ label, href: currentPath });
  }

  if (crumbs.length <= 1) return null;

  return (
    <nav className="flex items-center gap-1 text-xs text-muted-foreground mb-4">
      <Link href="/projects" className="hover:text-foreground transition-colors">
        <Home className="h-3 w-3" />
      </Link>
      {crumbs.map((crumb, i) => (
        <span key={crumb.href} className="flex items-center gap-1">
          <ChevronRight className="h-3 w-3 text-white/10" />
          {i === crumbs.length - 1 ? (
            <span className="text-foreground">{crumb.label}</span>
          ) : (
            <Link href={crumb.href} className="hover:text-foreground transition-colors">
              {crumb.label}
            </Link>
          )}
        </span>
      ))}
    </nav>
  );
}
