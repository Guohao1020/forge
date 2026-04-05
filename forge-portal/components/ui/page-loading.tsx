interface PageLoadingProps {
  rows?: number;
  height?: string;
}

export function PageLoading({ rows = 3, height = "h-24" }: PageLoadingProps) {
  return (
    <div className="space-y-4">
      {Array.from({ length: rows }).map((_, i) => (
        <div key={i} className={`${height} rounded-xl bg-white/5 animate-pulse`} />
      ))}
    </div>
  );
}
