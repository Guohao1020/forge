import type { Metadata } from "next";
import { Inter, JetBrains_Mono } from "next/font/google";
import { AuthProvider } from "@/lib/auth";
import { ErrorBoundary } from "@/components/error-boundary";
import { LoadingBar } from "@/components/ui/loading-bar";
import { KeyboardShortcutsDialog } from "@/components/keyboard-shortcuts";
import { Toaster } from "sonner";
import "./globals.css";

// Mockup: variant-B-dense.html lines 62-63. Inter body, JetBrains Mono code.
// Dense Engineering / Cursor IDE aesthetic.
const inter = Inter({
  variable: "--font-inter",
  subsets: ["latin"],
});

const jetbrainsMono = JetBrains_Mono({
  variable: "--font-jetbrains-mono",
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
    <html lang="zh-CN" className={`${inter.variable} ${jetbrainsMono.variable}`}>
      <body className="antialiased">
        <LoadingBar />
        <KeyboardShortcutsDialog />
        <ErrorBoundary>
          <AuthProvider>{children}</AuthProvider>
        </ErrorBoundary>
        <Toaster
          position="top-right"
          theme="light"
          richColors
        />
      </body>
    </html>
  );
}
