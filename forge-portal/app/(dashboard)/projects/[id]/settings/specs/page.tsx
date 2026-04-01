"use client";

import { useState, useEffect, useCallback } from "react";
import { useParams } from "next/navigation";
import {
  ChevronDown,
  ChevronRight,
  BookOpen,
  ShieldCheck,
  Building2,
  FolderGit2,
  Copy,
} from "lucide-react";
import { Button } from "@/components/ui/button";
import {
  Collapsible,
  CollapsibleContent,
  CollapsibleTrigger,
} from "@/components/ui/collapsible";
import {
  Standard,
  ReviewRule,
  EffectiveSpecs,
  getEffectiveSpecs,
  createStandard,
  createReviewRule,
} from "@/lib/specs";

const CATEGORY_COLORS: Record<string, string> = {
  JAVA: "bg-orange-500/10 text-orange-400",
  SQL: "bg-blue-500/10 text-blue-400",
  REDIS: "bg-red-500/10 text-red-400",
  KAFKA: "bg-green-500/10 text-green-400",
  API: "bg-purple-500/10 text-purple-400",
  SECURITY: "bg-yellow-500/10 text-yellow-400",
  NAMING: "bg-cyan-500/10 text-cyan-400",
  GIT: "bg-pink-500/10 text-pink-400",
};

const SEVERITY_COLORS: Record<string, string> = {
  ERROR: "bg-red-500/10 text-red-400",
  WARNING: "bg-yellow-500/10 text-yellow-400",
  INFO: "bg-blue-500/10 text-blue-400",
};

export default function ProjectSpecsPage() {
  const params = useParams();
  const projectId = Number(params.id);

  const [effectiveSpecs, setEffectiveSpecs] = useState<EffectiveSpecs | null>(null);
  const [loading, setLoading] = useState(true);
  const [expandedStandards, setExpandedStandards] = useState<Set<number>>(new Set());
  const [expandedRules, setExpandedRules] = useState<Set<number>>(new Set());

  const fetchSpecs = useCallback(async () => {
    setLoading(true);
    try {
      const result = await getEffectiveSpecs(projectId);
      setEffectiveSpecs(result);
    } catch (err) {
      console.error("Failed to fetch effective specs:", err);
    } finally {
      setLoading(false);
    }
  }, [projectId]);

  useEffect(() => {
    fetchSpecs();
  }, [fetchSpecs]);

  const toggleStandard = (id: number) => {
    setExpandedStandards((prev) => {
      const next = new Set(prev);
      if (next.has(id)) next.delete(id);
      else next.add(id);
      return next;
    });
  };

  const toggleRule = (id: number) => {
    setExpandedRules((prev) => {
      const next = new Set(prev);
      if (next.has(id)) next.delete(id);
      else next.add(id);
      return next;
    });
  };

  const handleOverrideStandard = async (std: Standard) => {
    if (!confirm(`确定要为此项目创建"${std.name}"的覆盖副本吗？`)) return;
    try {
      await createStandard({
        name: std.name,
        category: std.category,
        scope: "PROJECT",
        scopeId: projectId,
        parentId: std.id,
        content: std.content,
      });
      fetchSpecs();
    } catch (err) {
      console.error("Failed to override standard:", err);
    }
  };

  const handleOverrideRule = async (rule: ReviewRule) => {
    if (!confirm(`确定要为此项目创建"${rule.name}"的覆盖副本吗？`)) return;
    try {
      await createReviewRule({
        name: rule.name,
        category: rule.category,
        scope: "PROJECT",
        scopeId: projectId,
        ruleType: rule.ruleType,
        definition: rule.definition,
        severity: rule.severity,
        autoFix: rule.autoFix,
        fixTemplate: rule.fixTemplate || undefined,
      });
      fetchSpecs();
    } catch (err) {
      console.error("Failed to override rule:", err);
    }
  };

  const ScopeIcon = ({ scope }: { scope: string }) =>
    scope === "COMPANY" ? (
      <Building2 className="h-3.5 w-3.5 text-[#8B5CF6]" />
    ) : (
      <FolderGit2 className="h-3.5 w-3.5 text-green-400" />
    );

  const ScopeLabel = ({ scope }: { scope: string }) => (
    <span className={`text-xs ${scope === "COMPANY" ? "text-[#8B5CF6]" : "text-green-400"}`}>
      {scope === "COMPANY" ? "公司级" : "项目级"}
    </span>
  );

  if (loading) {
    return (
      <div className="flex items-center justify-center h-64 text-white/30">
        加载中...
      </div>
    );
  }

  return (
    <div className="space-y-8 max-w-4xl">
      <div>
        <h2 className="text-xl font-bold text-white mb-1">项目规范配置</h2>
        <p className="text-sm text-white/50">
          查看此项目的有效规范。公司级规范自动继承，点击&ldquo;Override&rdquo;可创建项目级覆盖副本。
        </p>
      </div>

      {/* Standards Section */}
      <div className="space-y-3">
        <div className="flex items-center gap-2 text-white/70">
          <BookOpen className="h-5 w-5" />
          <h3 className="text-lg font-semibold">编码规范</h3>
          <span className="text-sm text-white/30">
            ({effectiveSpecs?.standards?.length || 0})
          </span>
        </div>

        {effectiveSpecs?.standards?.length === 0 ? (
          <div className="text-sm text-white/30 py-4">暂无生效的编码规范</div>
        ) : (
          <div className="space-y-2">
            {effectiveSpecs?.standards?.map((std) => (
              <Collapsible
                key={std.id}
                open={expandedStandards.has(std.id)}
                onOpenChange={() => toggleStandard(std.id)}
              >
                <div className="bg-white/[0.03] border border-white/10 rounded-lg">
                  <CollapsibleTrigger className="flex items-center justify-between w-full px-4 py-3 text-left">
                    <div className="flex items-center gap-3">
                      {expandedStandards.has(std.id) ? (
                        <ChevronDown className="h-4 w-4 text-white/30" />
                      ) : (
                        <ChevronRight className="h-4 w-4 text-white/30" />
                      )}
                      <span className="text-white font-medium">{std.name}</span>
                      <span className={`px-2 py-0.5 rounded text-xs font-medium ${CATEGORY_COLORS[std.category] || "bg-white/10 text-white/50"}`}>
                        {std.category}
                      </span>
                      <ScopeIcon scope={std.scope} />
                      <ScopeLabel scope={std.scope} />
                    </div>
                    {std.scope === "COMPANY" && (
                      <Button
                        variant="ghost"
                        size="sm"
                        className="text-[#8B5CF6] hover:text-[#7C3AED] hover:bg-[#8B5CF6]/10"
                        onClick={(e) => {
                          e.stopPropagation();
                          handleOverrideStandard(std);
                        }}
                      >
                        <Copy className="h-3.5 w-3.5 mr-1.5" />
                        Override
                      </Button>
                    )}
                  </CollapsibleTrigger>
                  <CollapsibleContent>
                    <div className="px-4 pb-4 pt-1 border-t border-white/5">
                      <pre className="bg-[#0A0A12] rounded-lg p-4 text-sm text-white/70 font-mono whitespace-pre-wrap overflow-auto max-h-[400px]">
                        {std.content}
                      </pre>
                    </div>
                  </CollapsibleContent>
                </div>
              </Collapsible>
            ))}
          </div>
        )}
      </div>

      {/* Rules Section */}
      <div className="space-y-3">
        <div className="flex items-center gap-2 text-white/70">
          <ShieldCheck className="h-5 w-5" />
          <h3 className="text-lg font-semibold">Review 规则</h3>
          <span className="text-sm text-white/30">
            ({effectiveSpecs?.rules?.length || 0})
          </span>
        </div>

        {effectiveSpecs?.rules?.length === 0 ? (
          <div className="text-sm text-white/30 py-4">暂无生效的 Review 规则</div>
        ) : (
          <div className="space-y-2">
            {effectiveSpecs?.rules?.map((rule) => (
              <Collapsible
                key={rule.id}
                open={expandedRules.has(rule.id)}
                onOpenChange={() => toggleRule(rule.id)}
              >
                <div className="bg-white/[0.03] border border-white/10 rounded-lg">
                  <CollapsibleTrigger className="flex items-center justify-between w-full px-4 py-3 text-left">
                    <div className="flex items-center gap-3">
                      {expandedRules.has(rule.id) ? (
                        <ChevronDown className="h-4 w-4 text-white/30" />
                      ) : (
                        <ChevronRight className="h-4 w-4 text-white/30" />
                      )}
                      <span className="text-white font-medium">{rule.name}</span>
                      <span className={`px-2 py-0.5 rounded text-xs font-medium ${CATEGORY_COLORS[rule.category] || "bg-white/10 text-white/50"}`}>
                        {rule.category}
                      </span>
                      <span className={`px-2 py-0.5 rounded text-xs font-medium ${SEVERITY_COLORS[rule.severity] || "bg-white/10 text-white/50"}`}>
                        {rule.severity}
                      </span>
                      <ScopeIcon scope={rule.scope} />
                      <ScopeLabel scope={rule.scope} />
                    </div>
                    {rule.scope === "COMPANY" && (
                      <Button
                        variant="ghost"
                        size="sm"
                        className="text-[#8B5CF6] hover:text-[#7C3AED] hover:bg-[#8B5CF6]/10"
                        onClick={(e) => {
                          e.stopPropagation();
                          handleOverrideRule(rule);
                        }}
                      >
                        <Copy className="h-3.5 w-3.5 mr-1.5" />
                        Override
                      </Button>
                    )}
                  </CollapsibleTrigger>
                  <CollapsibleContent>
                    <div className="px-4 pb-4 pt-1 border-t border-white/5 space-y-3">
                      <div>
                        <span className="text-xs text-white/30">规则定义</span>
                        <pre className="bg-[#0A0A12] rounded-lg p-3 text-sm text-white/70 font-mono whitespace-pre-wrap mt-1">
                          {JSON.stringify(rule.definition, null, 2)}
                        </pre>
                      </div>
                      {rule.fixTemplate && (
                        <div>
                          <span className="text-xs text-white/30">修复模板</span>
                          <pre className="bg-[#0A0A12] rounded-lg p-3 text-sm text-white/70 font-mono whitespace-pre-wrap mt-1">
                            {rule.fixTemplate}
                          </pre>
                        </div>
                      )}
                    </div>
                  </CollapsibleContent>
                </div>
              </Collapsible>
            ))}
          </div>
        )}
      </div>
    </div>
  );
}
