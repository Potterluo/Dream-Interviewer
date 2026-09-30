"use client";

import { createContext, useContext } from "react";
import type { User } from "@/lib/api";

// user-context: the authenticated user, resolved once by AuthGuard and
// shared app-wide. Pages read it via useUser() instead of re-fetching
// /api/me. `refresh` re-reads after profile changes / login.

export interface UserContextValue {
  user: User | null;
  refresh: () => Promise<void>;
}

export const UserContext = createContext<UserContextValue>({
  user: null,
  refresh: async () => {},
});

export function useUser(): UserContextValue {
  return useContext(UserContext);
}

export function isAdmin(user: User | null): boolean {
  return user?.role === "admin";
}
