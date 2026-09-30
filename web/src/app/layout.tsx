import type { Metadata } from "next";
import { ThemeProvider } from "@/components/theme-provider";
import { AuthGuard } from "@/components/auth-guard";
import { AppShell } from "@/components/app-shell";
import "./globals.css";

export const metadata: Metadata = {
  title: "Dream Interviewer",
  description: "AI 面试官 —— 生成面试计划、逐题追问、评分与提升建议",
};

export default function RootLayout({
  children,
}: Readonly<{ children: React.ReactNode }>) {
  return (
    <html lang="zh-CN" suppressHydrationWarning>
      <head>
        {/* Apply the stored theme (mode + accent preset) before first
            paint — without this, users get a white flash / accent snap on
            every hard load. Keep in sync with theme-provider.tsx
            (THEME_KEY / ACCENT_KEY / DEFAULT_ACCENT). */}
        <script
          dangerouslySetInnerHTML={{
            __html: `(function(){try{var t=localStorage.getItem('app-theme');if(t==='dark'||((!t||t==='system')&&window.matchMedia('(prefers-color-scheme: dark)').matches)){document.documentElement.classList.add('dark')}var a=localStorage.getItem('app-accent');if(a&&a!=='violet'){document.documentElement.setAttribute('data-theme',a)}}catch(e){}})()`,
          }}
        />
      </head>
      <body className="antialiased">
        <ThemeProvider>
          <AuthGuard>
            <AppShell>{children}</AppShell>
          </AuthGuard>
        </ThemeProvider>
      </body>
    </html>
  );
}
