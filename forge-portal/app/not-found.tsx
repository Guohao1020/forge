import Link from "next/link";

export default function NotFound() {
  return (
    <div className="min-h-screen flex items-center justify-center bg-background">
      <div className="text-center max-w-md">
        <div className="text-8xl font-bold text-primary/20 mb-4">404</div>
        <h1 className="text-2xl font-semibold text-foreground mb-2">页面未找到</h1>
        <p className="text-sm text-muted-foreground mb-8">
          你访问的页面不存在或已被移除。
        </p>
        <Link
          href="/projects"
          className="inline-flex items-center px-6 py-2.5 bg-primary text-white rounded-lg text-sm font-medium hover:bg-primary/90 transition-colors"
        >
          返回项目大厅
        </Link>
      </div>
    </div>
  );
}
