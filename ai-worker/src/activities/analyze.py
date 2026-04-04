import logging
from dataclasses import dataclass, field
from typing import Any, Dict, List, Optional
from temporalio import activity
from src.agents.analyst import AnalystAgent
from src.context.cache import ContextCache
from src.models.router import ModelRouter

logger = logging.getLogger(__name__)

@dataclass
class AnalyzeInput:
    project_id: int
    task_id: int
    requirement: str
    conversation_history: Optional[List[Dict[str, Any]]] = None

@dataclass
class AnalyzeOutput:
    status: str          # "clarify" | "confirmed"
    content: str         # Raw AI response text
    metadata: Dict[str, Any]  # Structured data
    tokens_used: int
    model: str
    provider: str
    latency_ms: int
    risks: List[Dict[str, str]] = field(default_factory=list)

def format_human_response(status: str, structured: dict) -> str:
    """Convert structured AI JSON response to human-readable Chinese markdown.

    Handles two output formats:
    - New format: single `question` + optional `options` array
    - Legacy format: `questions` array + `partial_summary`
    """
    if status == "clarify":
        parts = []

        # Understanding section
        understanding = structured.get("understanding") or structured.get("partial_summary")
        if understanding:
            parts.append(f"**💡 我的理解：**\n{understanding}")

        # Phase indicator
        phase_labels = {
            "understanding": "📋 初步理解",
            "scenario": "🔍 场景澄清",
            "constraints": "⚙️ 约束确认",
        }
        phase = structured.get("phase", "")
        if phase and phase in phase_labels:
            parts.append(f"*当前阶段：{phase_labels[phase]}*")

        # Single question (new format)
        question = structured.get("question")
        if question:
            parts.append(f"**❓ {question}**")

        # Options are NOT rendered in text — they're rendered as clickable buttons
        # in the frontend OptionButtons component. Only show as text fallback
        # if the frontend can't render buttons (e.g., old messages from DB).
        # The frontend will detect options in metadata and render buttons instead.

        # Risks (only show if non-empty)
        risks = structured.get("risks", [])
        if risks:
            risk_lines = []
            for r in risks:
                level = r.get("level", "MEDIUM")
                desc = r.get("description", "")
                mitigation = r.get("mitigation", "")
                emoji = {"HIGH": "🔴", "MEDIUM": "🟡", "LOW": "🟢"}.get(level, "⚪")
                risk_lines.append(f"- {emoji} **[{level}]** {desc}")
                if mitigation:
                    risk_lines.append(f"  └ 缓解：{mitigation}")
            parts.append("**⚠️ 风险提示：**\n" + "\n".join(risk_lines))

        return "\n\n".join(parts) if parts else structured.get("understanding", str(structured))

    elif status == "confirmed":
        parts = []

        parts.append("## ✅ 需求确认\n")

        summary = structured.get("summary")
        if summary:
            parts.append(f"{summary}\n")

        # Functional requirements
        func_reqs = structured.get("functional_requirements", [])
        if func_reqs:
            parts.append("### 功能需求")
            for i, req in enumerate(func_reqs, 1):
                parts.append(f"{i}. {req}")

        # Non-functional
        nf = structured.get("non_functional", {})
        if nf and isinstance(nf, dict):
            nf_items = []
            labels = {"performance": "性能", "security": "安全", "compatibility": "兼容性"}
            for k, v in nf.items():
                if v:
                    label = labels.get(k, k)
                    nf_items.append(f"- **{label}：** {v}")
            if nf_items:
                parts.append("\n### 非功能需求\n" + "\n".join(nf_items))

        # Modules & complexity
        modules = structured.get("affected_modules", [])
        complexity = structured.get("estimated_complexity", "")
        if modules or complexity:
            info = []
            if modules:
                info.append(f"**影响模块：** {', '.join(modules)}")
            if complexity:
                emoji = {"LOW": "🟢", "MEDIUM": "🟡", "HIGH": "🔴"}.get(complexity, "")
                info.append(f"**复杂度：** {emoji} {complexity}")
            parts.append("\n" + " | ".join(info))

        # Acceptance criteria
        criteria = structured.get("acceptance_criteria", [])
        if criteria:
            parts.append("\n### 验收标准")
            for i, c in enumerate(criteria, 1):
                parts.append(f"{i}. {c}")

        # Out of scope
        oos = structured.get("out_of_scope", [])
        if oos:
            parts.append("\n### 不在范围内")
            for item in oos:
                parts.append(f"- {item}")

        # Risks
        risks = structured.get("risks", [])
        if risks:
            risk_lines = []
            for r in risks:
                level = r.get("level", "MEDIUM")
                desc = r.get("description", "")
                emoji = {"HIGH": "🔴", "MEDIUM": "🟡", "LOW": "🟢"}.get(level, "⚪")
                risk_lines.append(f"- {emoji} **[{level}]** {desc}")
            parts.append("\n### 风险识别\n" + "\n".join(risk_lines))

        parts.append("\n---\n*请确认以上需求，确认后将开始方案规划。*")
        return "\n".join(parts)

    return structured.get("summary", str(structured))


def normalize_clarify_response(structured: dict) -> dict:
    """Normalize AI output to ensure single-question format with options.

    If the model returns legacy format (questions array), convert to
    new format (single question + options). Also ensures options are
    always present for clarify responses.
    """
    if structured.get("status") != "clarify":
        return structured

    # Legacy: questions array → pick first as main question, rest as options
    questions = structured.get("questions", [])
    if questions:
        if "question" not in structured or not structured["question"]:
            structured["question"] = questions[0]
        # If remaining questions exist and no options provided,
        # convert them into options for the user to click
        if "options" not in structured and len(questions) > 1:
            structured["options"] = questions[1:4]  # max 3 options from remaining
        if "questions" in structured:
            del structured["questions"]

    # Ensure phase exists
    if "phase" not in structured:
        structured["phase"] = "understanding"

    # Normalize partial_summary → understanding
    if "partial_summary" in structured and "understanding" not in structured:
        structured["understanding"] = structured.pop("partial_summary")

    # CRITICAL: If no options provided, try to auto-generate from question context
    # This ensures the frontend always has clickable options to display
    if not structured.get("options"):
        question = structured.get("question", "")
        structured["options"] = _generate_fallback_options(question)

    return structured


def _generate_fallback_options(question: str) -> list:
    """Generate sensible fallback options when AI doesn't provide them.

    Scans the question text for embedded choices (e.g. "A还是B")
    or generates generic yes/no/other options.
    """
    q = question.lower()

    # Pattern: "A还是B" / "A或者B" / "A或B"
    for sep in ["还是", "或者", "或"]:
        if sep in question:
            parts = question.split("？")[0].split("?")[0]  # before question mark
            # Find the clause containing the separator
            for clause in [parts]:
                if sep in clause:
                    # Try to extract options around separator
                    segments = clause.split(sep)
                    if len(segments) >= 2:
                        opts = [s.strip().rstrip("，。、") for s in segments]
                        opts = [o for o in opts if len(o) > 1 and len(o) < 50]
                        if len(opts) >= 2:
                            return opts[:4]

    # Pattern: question about yes/no type
    if any(kw in q for kw in ["是否", "需不需要", "要不要", "需要吗", "吗？", "吗?"]):
        return ["是的，需要", "不需要", "暂不确定，稍后决定"]

    # Pattern: question about scale/scope
    if any(kw in q for kw in ["多少", "多大", "几个", "多长"]):
        return ["小规模（少量用户）", "中等规模（百级用户）", "大规模（千级以上）"]

    # Pattern: question about features/functionality
    if any(kw in q for kw in ["功能", "特性", "支持", "具备"]):
        return ["基础功能即可", "需要较完整的功能", "需要高级/专业功能"]

    # Generic fallback
    return ["是的", "不是", "需要进一步讨论"]


@activity.defn(name="analyze_requirement")
async def analyze_requirement_activity(input: AnalyzeInput) -> AnalyzeOutput:
    logger.info(f"Analyzing requirement for task {input.task_id}")
    cache = ContextCache()
    try:
        ctx = await cache.get_or_build(
            project_id=input.project_id,
            purpose="requirement-analysis",
            conversation_history=input.conversation_history,
        )
        router = ModelRouter()
        agent = AnalystAgent(router)
        result = await agent.run(input.requirement, ctx)

        # Normalize to new single-question format
        result.structured = normalize_clarify_response(result.structured)

        status = result.structured.get("status", "clarify")
        risks = result.structured.get("risks", [])
        human_content = format_human_response(status, result.structured)
        return AnalyzeOutput(
            status=status,
            content=human_content,
            metadata=result.structured,
            tokens_used=result.tokens_used,
            model=result.model,
            provider=result.provider,
            latency_ms=result.latency_ms,
            risks=risks,
        )
    finally:
        await cache.close()
