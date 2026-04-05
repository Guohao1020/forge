import { type LucideIcon } from "lucide-react";

interface EmptyStateProps {
  icon: LucideIcon;
  title: string;
  description?: string;
  action?: React.ReactNode;
}

export function EmptyState({ icon: Icon, title, description, action }: EmptyStateProps) {
  return (
    <div className="rounded-xl border border-border bg-card p-12 text-center">
      <Icon className="h-12 w-12 text-muted-foreground mx-auto mb-4 opacity-30" />
      <h3 className="text-lg font-medium text-foreground mb-1">{title}</h3>
      {description && (
        <p className="text-sm text-muted-foreground mb-6">{description}</p>
      )}
      {action}
    </div>
  );
}
