import { CheckCircle2, XCircle, Loader2, Clock, Pause, AlertTriangle } from "lucide-react";

const STATUS_CONFIG: Record<string, { label: string; color: string; icon: typeof CheckCircle2 }> = {
  SUBMITTED: { label: "已提交", color: "text-blue-400 bg-blue-500/10 border-blue-500/20", icon: Clock },
  ANALYZING: { label: "分析中", color: "text-purple-400 bg-purple-500/10 border-purple-500/20", icon: Loader2 },
  PLANNING: { label: "规划中", color: "text-indigo-400 bg-indigo-500/10 border-indigo-500/20", icon: Loader2 },
  RUNNING: { label: "执行中", color: "text-amber-400 bg-amber-500/10 border-amber-500/20", icon: Loader2 },
  COMPLETED: { label: "已完成", color: "text-green-400 bg-green-500/10 border-green-500/20", icon: CheckCircle2 },
  FAILED: { label: "失败", color: "text-red-400 bg-red-500/10 border-red-500/20", icon: XCircle },
  CANCELLED: { label: "已取消", color: "text-gray-400 bg-gray-500/10 border-gray-500/20", icon: Pause },
  DEPLOYING: { label: "部署中", color: "text-cyan-400 bg-cyan-500/10 border-cyan-500/20", icon: Loader2 },
  // Version statuses
  IN_PROGRESS: { label: "进行中", color: "text-amber-400 bg-amber-500/10 border-amber-500/20", icon: Loader2 },
  TESTING: { label: "测试中", color: "text-yellow-400 bg-yellow-500/10 border-yellow-500/20", icon: AlertTriangle },
  RELEASED: { label: "已发布", color: "text-green-400 bg-green-500/10 border-green-500/20", icon: CheckCircle2 },
  ACTIVE: { label: "活跃", color: "text-green-400 bg-green-500/10 border-green-500/20", icon: CheckCircle2 },
  ARCHIVED: { label: "已归档", color: "text-gray-400 bg-gray-500/10 border-gray-500/20", icon: Pause },
};

interface StatusBadgeProps {
  status: string;
  size?: "sm" | "md";
}

export function StatusBadge({ status, size = "sm" }: StatusBadgeProps) {
  const config = STATUS_CONFIG[status] || {
    label: status,
    color: "text-gray-400 bg-gray-500/10 border-gray-500/20",
    icon: Clock,
  };
  const Icon = config.icon;
  const isAnimated = ["ANALYZING", "PLANNING", "RUNNING", "DEPLOYING", "IN_PROGRESS", "TESTING"].includes(status);

  return (
    <span className={`inline-flex items-center gap-1 px-2 py-0.5 rounded border ${config.color} ${
      size === "sm" ? "text-[10px]" : "text-xs"
    }`}>
      <Icon className={`${size === "sm" ? "h-2.5 w-2.5" : "h-3 w-3"} ${isAnimated ? "animate-spin" : ""}`} />
      {config.label}
    </span>
  );
}

export { STATUS_CONFIG };
