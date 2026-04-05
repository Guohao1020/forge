import type { Metadata } from "next";
import { Geist, Geist_Mono } from "next/font/google";
import { AuthProvider } from "@/lib/auth";
import { ErrorBoundary } from "@/components/error-boundary";
import { LoadingBar } from "@/components/ui/loading-bar";
import { KeyboardShortcutsDialog } from "@/components/keyboard-shortcuts";
import { Toaster } from "sonner";
import "./globals.css";

const geistSans = Geist({
  variable: "--font-geist-sans",
  subsets: ["latin"],
});

const geistMono = Geist_Mono({
  variable: "--font-geist-mono",
  subsets: ["latin"],
});

export const metadata: Metadata = {
  title: "Forge — Harness Engineering Platform",
  description: "AI-driven Harness Engineering Platform — 规范约束 + 机械化验证 + 可观测性闭环",
  icons: {
    icon: [
      { url: "/favicon.svg", type: "image/svg+xml" },
    ],
  },
  openGraph: {
    title: "Forge — Harness Engineering Platform",
    description: "AI-driven code generation with engineering constraints",
    type: "website",
  },
};

export default function RootLayout({
  children,
}: Readonly<{
  children: React.ReactNode;
}>) {
  return (
    <html lang="zh-CN" className={`${geistSans.variable} ${geistMono.variable} dark`}>
      <body className="antialiased">
        <LoadingBar />
        <KeyboardShortcutsDialog />
        <ErrorBoundary>
          <AuthProvider>{children}</AuthProvider>
        </ErrorBoundary>
        <Toaster
          position="top-right"
          theme="dark"
          richColors
        />
      </body>
    </html>
  );
}
