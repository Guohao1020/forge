"use client";

import { usePathname, useRouter } from "next/navigation";
import { BookOpen, MessageSquareCode, ShieldCheck, Boxes } from "lucide-react";
import { cn } from "@/lib/utils";

const tabs = [
  { label: "编码规范", href: "/specs/standards", icon: BookOpen },
  { label: "Prompt 模板", href: "/specs/prompts", icon: MessageSquareCode },
  { label: "Review 规则", href: "/specs/rules", icon: ShieldCheck },
  { label: "脚手架模板", href: "/specs/scaffolds", icon: Boxes },
];

export default function SpecsLayout({ children }: { children: React.ReactNode }) {
  const pathname = usePathname();
  const router = useRouter();

  return (
    <div className="flex flex-col h-full">
      {/* Header */}
      <div className="border-b border-white/10 px-6 pt-6 pb-0">
        <h1 className="text-2xl font-bold text-white mb-1">规范中心</h1>
        <p className="text-sm text-white/50 mb-4">
          管理编码规范、Prompt 模板、Review 规则和脚手架模板
        </p>

        {/* Tab navigation */}
        <div className="flex gap-1">
          {tabs.map((tab) => {
            const isActive = pathname.startsWith(tab.href);
            return (
              <button
                key={tab.href}
                onClick={() => router.push(tab.href)}
                className={cn(
                  "flex items-center gap-2 px-4 py-2.5 text-sm font-medium rounded-t-lg transition-colors",
                  isActive
                    ? "bg-white/10 text-white border-b-2 border-[#8B5CF6]"
                    : "text-white/50 hover:text-white/70 hover:bg-white/5"
                )}
              >
                <tab.icon className="h-4 w-4" />
                {tab.label}
              </button>
            );
          })}
        </div>
      </div>

      {/* Content */}
      <div className="flex-1 overflow-auto p-6">{children}</div>
    </div>
  );
}
