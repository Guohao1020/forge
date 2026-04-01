"use client";

import { useState, useEffect, useCallback } from "react";
import {
  Code2,
  Globe,
  Layers,
  Package,
  FileCode,
  ExternalLink,
  Variable,
  Terminal,
} from "lucide-react";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { ScaffoldTemplate, listScaffoldTemplates } from "@/lib/specs";

const PROJECT_TYPES = [
  { value: "all", label: "全部类型" },
  { value: "JAVA_MICROSERVICE", label: "Java 微服务" },
  { value: "VUE_FRONTEND", label: "Vue 3 前端" },
  { value: "FULLSTACK", label: "全栈项目" },
  { value: "SDK", label: "SDK 项目" },
  { value: "BLANK", label: "空白项目" },
];

const PROJECT_TYPE_LABELS: Record<string, string> = {
  JAVA_MICROSERVICE: "Java 微服务",
  VUE_FRONTEND: "Vue 3 前端",
  FULLSTACK: "全栈项目",
  SDK: "SDK 项目",
  BLANK: "空白项目",
};

const PROJECT_TYPE_COLORS: Record<string, string> = {
  JAVA_MICROSERVICE: "bg-orange-500/10 text-orange-400",
  VUE_FRONTEND: "bg-green-500/10 text-green-400",
  FULLSTACK: "bg-blue-500/10 text-blue-400",
  SDK: "bg-purple-500/10 text-purple-400",
  BLANK: "bg-white/10 text-white/60",
};

function ProjectTypeIcon({
  type,
  className,
}: {
  type: string;
  className?: string;
}) {
  const props = { className: className || "h-5 w-5" };
  switch (type) {
    case "JAVA_MICROSERVICE":
      return <Code2 {...props} />;
    case "VUE_FRONTEND":
      return <Globe {...props} />;
    case "FULLSTACK":
      return <Layers {...props} />;
    case "SDK":
      return <Package {...props} />;
    case "BLANK":
    default:
      return <FileCode {...props} />;
  }
}

export default function ScaffoldsPage() {
  const [templates, setTemplates] = useState<ScaffoldTemplate[]>([]);
  const [projectType, setProjectType] = useState("all");
  const [loading, setLoading] = useState(true);

  const fetchTemplates = useCallback(async () => {
    setLoading(true);
    try {
      const result = await listScaffoldTemplates({
        projectType: projectType === "all" ? undefined : projectType,
        pageSize: 100,
      });
      setTemplates(result.items || []);
    } catch (err) {
      console.error("Failed to fetch scaffold templates:", err);
    } finally {
      setLoading(false);
    }
  }, [projectType]);

  useEffect(() => {
    fetchTemplates();
  }, [fetchTemplates]);

  return (
    <div className="space-y-4">
      {/* Toolbar */}
      <div className="flex items-center gap-3">
        <Select
          value={projectType}
          onValueChange={(v) => setProjectType(v ?? "")}
        >
          <SelectTrigger className="w-[180px] bg-white/5 border-white/10 text-white">
            <SelectValue placeholder="全部类型" />
          </SelectTrigger>
          <SelectContent>
            {PROJECT_TYPES.map((t) => (
              <SelectItem key={t.value} value={t.value}>
                {t.label}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
      </div>

      {/* Card Grid */}
      {loading ? (
        <div className="flex items-center justify-center py-20 text-white/30">
          加载中...
        </div>
      ) : templates.length === 0 ? (
        <div className="flex flex-col items-center justify-center py-20 text-white/30 space-y-2">
          <FileCode className="h-10 w-10 text-white/15" />
          <span>暂无脚手架模板</span>
        </div>
      ) : (
        <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
          {templates.map((tpl) => (
            <div
              key={tpl.id}
              className="bg-white/[0.03] border border-white/10 rounded-lg p-5 hover:bg-white/[0.05] transition-colors"
            >
              {/* Header: icon + name + badge */}
              <div className="flex items-start gap-3 mb-3">
                <div className="mt-0.5 text-white/60">
                  <ProjectTypeIcon type={tpl.projectType} />
                </div>
                <div className="flex-1 min-w-0">
                  <div className="flex items-center gap-2 flex-wrap">
                    <h3 className="text-white font-medium truncate">
                      {tpl.name}
                    </h3>
                    <span
                      className={`inline-flex px-2 py-0.5 rounded text-xs font-medium shrink-0 ${
                        PROJECT_TYPE_COLORS[tpl.projectType] ||
                        "bg-white/10 text-white/60"
                      }`}
                    >
                      {PROJECT_TYPE_LABELS[tpl.projectType] ||
                        tpl.projectType}
                    </span>
                  </div>
                  <span className="text-white/30 text-xs">
                    v{tpl.version}
                  </span>
                </div>
                {tpl.templateRepo && (
                  <a
                    href={tpl.templateRepo}
                    target="_blank"
                    rel="noopener noreferrer"
                    className="text-white/30 hover:text-[#8B5CF6] transition-colors shrink-0"
                    title="打开模板仓库"
                  >
                    <ExternalLink className="h-4 w-4" />
                  </a>
                )}
              </div>

              {/* Description */}
              {tpl.description && (
                <p className="text-white/50 text-sm mb-3 line-clamp-2">
                  {tpl.description}
                </p>
              )}

              {/* Variables */}
              {tpl.variables && tpl.variables.length > 0 && (
                <div className="flex items-center gap-1.5 flex-wrap mb-2">
                  <Variable className="h-3.5 w-3.5 text-white/25 shrink-0" />
                  {tpl.variables.map((v) => (
                    <span
                      key={v}
                      className="inline-flex items-center px-2 py-0.5 rounded-full bg-[#8B5CF6]/10 text-[#8B5CF6] text-xs"
                    >
                      {v}
                    </span>
                  ))}
                </div>
              )}

              {/* Post hooks */}
              {tpl.postHooks && tpl.postHooks.length > 0 && (
                <div className="flex items-center gap-1.5 flex-wrap">
                  <Terminal className="h-3.5 w-3.5 text-white/25 shrink-0" />
                  {tpl.postHooks.map((hook) => (
                    <span
                      key={hook}
                      className="inline-flex items-center px-2 py-0.5 rounded bg-white/5 text-white/40 text-xs font-mono"
                    >
                      {hook}
                    </span>
                  ))}
                </div>
              )}
            </div>
          ))}
        </div>
      )}
    </div>
  );
}
