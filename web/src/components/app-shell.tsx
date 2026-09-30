"use client";

import { useState } from "react";
import Link from "next/link";
import { usePathname, useRouter } from "next/navigation";
import {
  BarChart3,
  Cpu,
  FolderOpen,
  KeyRound,
  LayoutDashboard,
  Layers,
  ListChecks,
  LogOut,
  Menu,
  Moon,
  PlusCircle,
  Settings,
  SlidersHorizontal,
  Sun,
  Users,
  X,
  // --- gen:nav-icons ---
} from "lucide-react";
import { logout } from "@/lib/api";
import { useUser, isAdmin } from "./user-context";
import { useTheme } from "./theme-provider";
import { cn } from "@/lib/utils";
import { Button } from "@/components/ui/button";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuGroup,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";

// AppShell: sidebar navigation + topbar. The shell renders INSIDE
// AuthGuard, so `user` is always present here.

interface NavItem {
  href: string;
  label: string;
  icon: React.ComponentType<{ className?: string }>;
  adminOnly?: boolean;
}

const NAV: { section: string; items: NavItem[] }[] = [
  {
    section: "主导航",
    items: [
      { href: "/", label: "仪表盘", icon: LayoutDashboard },
      { href: "/interviews/", label: "面试记录", icon: ListChecks },
      { href: "/interviews/new/", label: "新建面试", icon: PlusCircle },
      { href: "/analytics/", label: "分析", icon: BarChart3 },
      { href: "/files/", label: "文件", icon: FolderOpen },
      // --- gen:nav ---
    ],
  },
  {
    section: "账号",
    items: [
      { href: "/settings/engine/", label: "引擎", icon: Cpu },
      { href: "/settings/apikeys/", label: "API 密钥", icon: KeyRound },
      { href: "/settings/", label: "设置", icon: Settings },
    ],
  },
  {
    section: "管理",
    items: [
      { href: "/admin/users/", label: "用户", icon: Users, adminOnly: true },
      { href: "/admin/presets/", label: "预置", icon: Layers, adminOnly: true },
      { href: "/admin/model/", label: "模型配置", icon: SlidersHorizontal, adminOnly: true },
    ],
  },
];

// Every registered href, longest first — used to make prefix matching
// unambiguous (/settings/apikeys must not also light up /settings).
const NAV_HREFS = NAV.flatMap((g) => g.items.map((i) => i.href.replace(/\/$/, ""))).sort(
  (a, b) => b.length - a.length
);

function isActive(pathname: string, href: string): boolean {
  const p = pathname.replace(/\/$/, "") || "/";
  const h = href.replace(/\/$/, "") || "/";
  if (p === h) return true;
  return p.startsWith(h + "/") && !NAV_HREFS.some((o) => o.length > h.length && p.startsWith(o));
}

export function AppShell({ children }: { children: React.ReactNode }) {
  const { user } = useUser();
  const pathname = usePathname();
  const [mobileOpen, setMobileOpen] = useState(false);

  const sidebar = (
    <nav className="flex h-full flex-col gap-6 p-4">
      <Link href="/" className="flex items-center gap-2 px-2 pt-2">
        <img src="/icon.png" alt="" className="h-8 w-8 rounded-lg shadow-sm" />
        <span className="font-heading text-base font-semibold">Dream Interviewer</span>
      </Link>
      {NAV.map((group) => {
        const items = group.items.filter((it) => !it.adminOnly || isAdmin(user));
        if (items.length === 0) return null;
        return (
          <div key={group.section}>
            <p className="mb-1 px-2 text-xs font-medium text-muted-foreground">{group.section}</p>
            <ul className="grid gap-0.5">
              {items.map((it) => (
                <li key={it.href}>
                  <Link
                    href={it.href}
                    onClick={() => setMobileOpen(false)}
                    className={cn(
                      "flex items-center gap-2.5 rounded-md px-2 py-1.5 text-sm transition-colors",
                      isActive(pathname, it.href)
                        ? "bg-sidebar-accent font-medium text-sidebar-accent-foreground"
                        : "text-muted-foreground hover:bg-sidebar-accent/60 hover:text-sidebar-accent-foreground"
                    )}
                  >
                    <it.icon className="h-4 w-4" />
                    {it.label}
                  </Link>
                </li>
              ))}
            </ul>
          </div>
        );
      })}
      <div className="mt-auto px-2 pb-1 text-xs text-muted-foreground">
        {user?.role === "admin" ? "已以管理员身份登录" : "已登录"}
      </div>
    </nav>
  );

  return (
    <div className="flex min-h-screen">
      {/* Desktop sidebar */}
      <aside className="sticky top-0 hidden h-screen w-60 shrink-0 border-r bg-sidebar text-sidebar-foreground md:block">
        {sidebar}
      </aside>

      {/* Mobile sidebar */}
      {mobileOpen && (
        <div className="fixed inset-0 z-50 md:hidden">
          <div
            className="absolute inset-0 bg-black/40"
            onClick={() => setMobileOpen(false)}
          />
          <aside className="absolute left-0 top-0 h-full w-64 border-r bg-sidebar">
            <button
              className="absolute right-2 top-2 rounded p-1.5 text-muted-foreground hover:bg-accent"
              onClick={() => setMobileOpen(false)}
              aria-label="关闭菜单"
            >
              <X className="h-4 w-4" />
            </button>
            {sidebar}
          </aside>
        </div>
      )}

      <div className="flex min-w-0 flex-1 flex-col">
        <Topbar onMenu={() => setMobileOpen(true)} />
        <main className="mx-auto w-full max-w-5xl flex-1 p-4 md:p-6">{children}</main>
      </div>
    </div>
  );
}

function Topbar({ onMenu }: { onMenu: () => void }) {
  const { user, refresh } = useUser();
  const { resolvedTheme, setTheme } = useTheme();
  const router = useRouter();

  async function doLogout() {
    await logout();
    await refresh();
    router.replace("/");
    // Static export: a hard reload re-runs the AuthGuard probe.
    if (typeof window !== "undefined") window.location.reload();
  }

  return (
    <header className="sticky top-0 z-40 flex h-14 items-center gap-3 border-b bg-background/80 px-4 backdrop-blur">
      <Button variant="ghost" size="icon" className="md:hidden" onClick={onMenu} aria-label="打开菜单">
        <Menu className="h-5 w-5" />
      </Button>
      <span className="font-heading text-sm font-semibold md:hidden">Dream Interviewer</span>
      <div className="flex-1" />
      <Button
        variant="ghost"
        size="icon"
        onClick={() => setTheme(resolvedTheme === "dark" ? "light" : "dark")}
        aria-label="切换主题"
      >
        {resolvedTheme === "dark" ? <Sun className="h-4.5 w-4.5" /> : <Moon className="h-4.5 w-4.5" />}
      </Button>
      <DropdownMenu>
        <DropdownMenuTrigger
          render={
            <Button variant="ghost" className="gap-2 px-2">
              <span className="flex h-7 w-7 items-center justify-center rounded-full bg-primary text-xs font-semibold text-primary-foreground">
                {(user?.displayName || user?.username || "?").slice(0, 1).toUpperCase()}
              </span>
              <span className="hidden text-sm sm:inline">{user?.displayName || user?.username}</span>
            </Button>
          }
        />
        <DropdownMenuContent align="end" className="w-52">
          {/* GroupLabel (DropdownMenuLabel) requires a Menu.Group parent. */}
          <DropdownMenuGroup>
            <DropdownMenuLabel>
              <p className="text-sm font-medium">{user?.displayName || user?.username}</p>
              <p className="text-xs font-normal text-muted-foreground">{user?.email}</p>
            </DropdownMenuLabel>
            <DropdownMenuSeparator />
            <DropdownMenuItem onClick={() => router.push("/settings/")}>
              <Settings className="h-4 w-4" /> 设置
            </DropdownMenuItem>
            <DropdownMenuSeparator />
            <DropdownMenuItem variant="destructive" onClick={doLogout}>
              <LogOut className="h-4 w-4" /> 退出登录
            </DropdownMenuItem>
          </DropdownMenuGroup>
        </DropdownMenuContent>
      </DropdownMenu>
    </header>
  );
}
