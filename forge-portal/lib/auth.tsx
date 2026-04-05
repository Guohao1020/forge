"use client";

import { createContext, useContext, useEffect, useState, useCallback } from "react";
import { useRouter } from "next/navigation";
import { api } from "./api";

interface UserInfo {
  id: number;
  tenant_id: number;
  username: string;
  display_name: string;
  avatar_url: string;
  roles: { id: number; code: string; name: string }[];
}

interface LoginResponse {
  token: string;
  expires_at: string;
  user: UserInfo;
}

interface AuthContextType {
  user: UserInfo | null;
  loading: boolean;
  login: (username: string, password: string) => Promise<void>;
  logout: () => Promise<void>;
}

const AuthContext = createContext<AuthContextType | null>(null);

export function AuthProvider({ children }: { children: React.ReactNode }) {
  const [user, setUser] = useState<UserInfo | null>(null);
  const [loading, setLoading] = useState(true);
  const router = useRouter();

  useEffect(() => {
    const token = localStorage.getItem("forge_token");
    const savedUser = localStorage.getItem("forge_user");
    if (token && savedUser) {
      try {
        setUser(JSON.parse(savedUser));
      } catch {
        localStorage.removeItem("forge_token");
        localStorage.removeItem("forge_user");
      }
    }
    setLoading(false);
  }, []);

  const login = useCallback(async (username: string, password: string) => {
    const data = await api.post<LoginResponse>("/auth/login", { username, password });
    localStorage.setItem("forge_token", data.token);
    localStorage.setItem("forge_user", JSON.stringify(data.user));
    setUser(data.user);
    router.push("/projects");
  }, [router]);

  const logout = useCallback(async () => {
    try {
      await api.post("/auth/logout");
    } catch {
      // ignore errors during logout
    }
    localStorage.removeItem("forge_token");
    localStorage.removeItem("forge_user");
    setUser(null);
    router.push("/login");
  }, [router]);

  return (
    <AuthContext.Provider value={{ user, loading, login, logout }}>
      {children}
    </AuthContext.Provider>
  );
}

export function useAuth() {
  const ctx = useContext(AuthContext);
  if (!ctx) throw new Error("useAuth must be used within AuthProvider");
  return ctx;
}
