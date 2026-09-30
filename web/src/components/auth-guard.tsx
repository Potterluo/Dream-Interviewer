"use client";

import { useCallback, useEffect, useState } from "react";
import { useRouter, usePathname } from "next/navigation";
import { getMe, getStatus, type User } from "@/lib/api";
import { LoginScreen } from "./login-screen";
import { UserContext } from "./user-context";

interface AuthGuardProps {
  children: React.ReactNode;
}

// Routes that require the admin role. The server enforces this
// authoritatively on every /api/admin/* and /api/users call; this client
// gate just stops non-admins from landing on a page that would render an
// empty shell. Keep the list in sync with the admin routes
// (docs/API.md §9): /admin/users, /admin/presets, /admin/model.
const ADMIN_PATH_PREFIXES = ["/admin", "/admin/presets", "/admin/model"];

function isAdminPath(pathname: string): boolean {
  return ADMIN_PATH_PREFIXES.some(
    (p) => pathname === p || pathname.startsWith(p + "/")
  );
}

// AuthGuard decides between three states on every navigation:
//
//   1. instance not configured (no users yet) → redirect to /onboard
//   2. no valid session                       → render LoginScreen inline
//   3. authenticated                          → render children
//
// The login screen is rendered inline instead of a /login route so the
// URL never changes and post-login the requested page is already
// mounted. The server remains the authority — every API call is checked
// there too.
export function AuthGuard({ children }: AuthGuardProps) {
  const router = useRouter();
  const pathname = usePathname();
  const [checked, setChecked] = useState(false);
  const [authed, setAuthed] = useState(false);
  const [user, setUser] = useState<User | null>(null);

  const refresh = useCallback(async () => {
    const me = await getMe();
    if (me.ok && me.user) {
      setUser(me.user);
      setAuthed(true);
    } else {
      setUser(null);
      setAuthed(false);
    }
  }, []);

  useEffect(() => {
    let aborted = false;
    (async () => {
      let configured = false;
      try {
        const status = await getStatus();
        configured = !!status.configured;
      } catch {
        // server unreachable — fall through to the login screen, which
        // will surface the error on submit
      }
      if (aborted) return;

      if (!configured) {
        const onOnboard = pathname === "/onboard" || pathname.startsWith("/onboard/");
        if (!onOnboard) {
          router.replace("/onboard/");
          return;
        }
        // The onboarding wizard runs pre-auth by design (no users exist
        // yet) — let it through unauthenticated.
        setAuthed(true);
        setChecked(true);
        return;
      }

      try {
        const me = await getMe();
        if (me.ok && me.user) {
          if (isAdminPath(pathname) && me.user.role !== "admin") {
            router.replace("/");
            return;
          }
          setUser(me.user);
          setAuthed(true);
        }
      } catch {
        // network failure — fall through to LoginScreen
      }
      if (!aborted) setChecked(true);
    })();
    return () => {
      aborted = true;
    };
  }, [router, pathname]);

  // Re-check admin paths whenever the signed-in USER changes (login as a
  // different account happens inline on whatever page was open — a
  // non-admin landing on an /admin page must be bounced out).
  useEffect(() => {
    if (user && isAdminPath(pathname) && user.role !== "admin") {
      router.replace("/");
    }
  }, [user, pathname, router]);

  const value = { user, refresh };

  if (!checked) {
    return (
      <div className="flex min-h-screen items-center justify-center bg-background">
        <div className="h-8 w-8 animate-spin rounded-full border-2 border-muted border-t-primary" />
      </div>
    );
  }
  return (
    <UserContext.Provider value={value}>
      {!authed ? (
        <LoginScreen
          onSuccess={async () => {
            await refresh();
          }}
        />
      ) : (
        children
      )}
    </UserContext.Provider>
  );
}
