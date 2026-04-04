"use client";

import { useMemo } from "react";
import { MarkdownPreview } from "@/components/markdown-preview";

interface MessageBubbleProps {
  role: string;
  content: string;
  createdAt?: string;
  metadata?: Record<string, unknown>;
}

/**
 * Detect if content is raw JSON from AI and convert to human-readable Chinese markdown.
 * Frontend fallback — backend should already format via format_human_response,
 * but old messages or edge cases may still contain raw JSON.
 * Supports both new format (single question + options) and legacy (questions array).
 */
function formatAIContent(content: string): string {
  const trimmed = content.trim();
  if (!trimmed.startsWith("{") && !trimmed.startsWith("[")) return content;

  try {
    const data = JSON.parse(trimmed);
    if (!data || typeof data !== "object") return content;

    const status = data.status;
    if (status === "clarify") {
      const parts: string[] = [];

      // Understanding
      const understanding = data.understanding || data.partial_summary;
      if (understanding) {
        parts.push(`**💡 我的理解：**\n${understanding}`);
      }

      // Phase indicator
      const phaseLabels: Record<string, string> = {
        understanding: "📋 初步理解",
        scenario: "🔍 场景澄清",
        constraints: "⚙️ 约束确认",
      };
      if (data.phase && phaseLabels[data.phase]) {
        parts.push(`*当前阶段：${phaseLabels[data.phase]}*`);
      }

      // Single question (new format)
      if (data.question) {
        parts.push(`**❓ ${data.question}**`);
      }

      // Options are rendered as clickable OptionButtons component — NOT as text
      // Legacy: multiple questions → just show first as question
      if (Array.isArray(data.questions) && data.questions.length > 0 && !data.question) {
        parts.push(`**❓ ${data.questions[0]}**`);
      }

      // Risks
      if (Array.isArray(data.risks) && data.risks.length > 0) {
        const emojiMap: Record<string, string> = { HIGH: "🔴", MEDIUM: "🟡", LOW: "🟢" };
        const riskLines = data.risks.map(
          (r: { level?: string; description?: string; mitigation?: string }) => {
            const lvl = r.level || "MEDIUM";
            const emoji = emojiMap[lvl] || "⚪";
            let line = `- ${emoji} **[${lvl}]** ${r.description || ""}`;
            if (r.mitigation) line += `\n  └ 缓解：${r.mitigation}`;
            return line;
          }
        );
        parts.push(`**⚠️ 风险提示：**\n${riskLines.join("\n")}`);
      }

      return parts.length > 0 ? parts.join("\n\n") : content;
    }

    if (status === "confirmed") {
      const parts: string[] = [];
      parts.push("## ✅ 需求确认\n");

      if (data.summary) parts.push(`${data.summary}\n`);

      // Functional requirements
      if (Array.isArray(data.functional_requirements) && data.functional_requirements.length > 0) {
        parts.push("### 功能需求");
        data.functional_requirements.forEach((req: string, i: number) => {
          parts.push(`${i + 1}. ${req}`);
        });
      }

      // Non-functional
      if (data.non_functional && typeof data.non_functional === "object") {
        const labels: Record<string, string> = { performance: "性能", security: "安全", compatibility: "兼容性" };
        const nfItems = Object.entries(data.non_functional)
          .filter(([, v]) => v)
          .map(([k, v]) => `- **${labels[k] || k}：** ${v}`)
          .join("\n");
        if (nfItems) parts.push(`\n### 非功能需求\n${nfItems}`);
      }

      // Modules & complexity
      const info: string[] = [];
      if (Array.isArray(data.affected_modules) && data.affected_modules.length > 0) {
        info.push(`**影响模块：** ${data.affected_modules.join(", ")}`);
      }
      if (data.estimated_complexity) {
        const complexityEmoji: Record<string, string> = { LOW: "🟢", MEDIUM: "🟡", HIGH: "🔴" };
        const emoji = complexityEmoji[data.estimated_complexity as string] || "";
        info.push(`**复杂度：** ${emoji} ${data.estimated_complexity}`);
      }
      if (info.length > 0) parts.push(`\n${info.join(" | ")}`);

      // Acceptance criteria
      if (Array.isArray(data.acceptance_criteria) && data.acceptance_criteria.length > 0) {
        parts.push("\n### 验收标准");
        data.acceptance_criteria.forEach((c: string, i: number) => {
          parts.push(`${i + 1}. ${c}`);
        });
      }

      // Out of scope
      if (Array.isArray(data.out_of_scope) && data.out_of_scope.length > 0) {
        parts.push("\n### 不在范围内");
        data.out_of_scope.forEach((item: string) => {
          parts.push(`- ${item}`);
        });
      }

      // Risks
      if (Array.isArray(data.risks) && data.risks.length > 0) {
        const riskEmojiMap: Record<string, string> = { HIGH: "🔴", MEDIUM: "🟡", LOW: "🟢" };
        const riskLines = data.risks.map(
          (r: { level?: string; description?: string }) => {
            const lvl = r.level || "MEDIUM";
            const emoji = riskEmojiMap[lvl] || "⚪";
            return `- ${emoji} **[${lvl}]** ${r.description || ""}`;
          }
        );
        parts.push(`\n### 风险识别\n${riskLines.join("\n")}`);
      }

      parts.push("\n---\n*请确认以上需求，确认后将开始方案规划。*");
      return parts.join("\n");
    }

    // Unknown JSON with content/summary field
    if (data.content && typeof data.content === "string") return data.content;
    if (data.summary && typeof data.summary === "string") return data.summary;
    return content;
  } catch {
    return content;
  }
}

export function MessageBubble({ role, content, createdAt, metadata }: MessageBubbleProps) {
  const displayContent = useMemo(
    () => (role === "assistant" ? formatAIContent(content) : content),
    [role, content]
  );

  if (role === "system") {
    return (
      <div className="flex justify-center py-2">
        <span className="text-xs text-white/30">{content}</span>
      </div>
    );
  }

  const isUser = role === "user";

  return (
    <div className={`flex ${isUser ? "justify-end" : "justify-start"} mb-4`}>
      <div
        className={`max-w-[75%] rounded-2xl px-4 py-3 ${
          isUser
            ? "bg-[#8B5CF6]/10 text-white rounded-br-md"
            : "bg-white/5 text-white/80 rounded-bl-md"
        }`}
      >
        {isUser ? (
          <p className="text-sm whitespace-pre-wrap">{displayContent}</p>
        ) : (
          <MarkdownPreview content={displayContent} className="text-sm" />
        )}
        <div className="flex items-center gap-2 mt-2">
          {!isUser && metadata && typeof metadata.model === "string" && (
            <span className="text-[10px] text-white/15 font-mono">
              {metadata.model.replace(/^(claude|gpt|qwen)/i, (m) => m.charAt(0).toUpperCase() + m.slice(1))}
            </span>
          )}
          {createdAt && (
            <span className="text-[10px] text-white/15">
              {new Date(createdAt).toLocaleTimeString()}
            </span>
          )}
        </div>
      </div>
    </div>
  );
}
