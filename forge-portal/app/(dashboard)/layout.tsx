"use client";

import { useAuth } from "@/lib/auth";
import { useRouter, usePathname } from "next/navigation";
import { useEffect } from "react";
import { Sidebar } from "@/components/sidebar";
import { Topbar } from "@/components/topbar";
import { Breadcrumb } from "@/components/breadcrumb";

// Project detail pages manage their own layout (ProjectSidebar + no Topbar)
const PROJECT_DETAIL_PATTERN = /^\/projects\/\d+/;

export default function DashboardLayout({ children }: { children: React.ReactNode }) {
  const { user, loading } = useAuth();
  const router = useRouter();
  const pathname = usePathname();

  useEffect(() => {
    if (!loading && !user) {
      router.push("/login");
    }
  }, [user, loading, router]);

  if (loading) {
    return (
      <div className="min-h-screen flex items-center justify-center bg-background">
        <div className="text-sm text-muted-foreground">加载中...</div>
      </div>
    );
  }

  if (!user) return null;

  // Project detail pages render their own full layout
  if (PROJECT_DETAIL_PATTERN.test(pathname)) {
    return <>{children}</>;
  }

  return (
    <div className="flex h-screen bg-background">
      <Sidebar />
      <div className="flex-1 flex flex-col overflow-hidden">
        <Topbar />
        <main className="flex-1 overflow-auto p-6">
          <Breadcrumb />
          {children}
        </main>
      </div>
    </div>
  );
}
