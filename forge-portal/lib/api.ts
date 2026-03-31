const BASE_URL = "/api";

interface ApiResult<T> {
  code: number;
  message: string;
  data: T;
}

class ApiError extends Error {
  constructor(public code: number, message: string) {
    super(message);
  }
}

async function request<T>(
  path: string,
  options: RequestInit = {}
): Promise<T> {
  const token = typeof window !== "undefined"
    ? localStorage.getItem("forge_token")
    : null;

  const headers: Record<string, string> = {
    "Content-Type": "application/json",
    ...((options.headers as Record<string, string>) || {}),
  };

  if (token) {
    headers["Authorization"] = `Bearer ${token}`;
  }

  const res = await fetch(`${BASE_URL}${path}`, {
    ...options,
    headers,
  });

  const text = await res.text();
  let json: ApiResult<T>;
  try {
    json = JSON.parse(text);
  } catch {
    throw new ApiError(-1, "服务器无响应，请确认后端已启动");
  }

  if (res.status === 401) {
    // For non-login 401s, redirect to login page
    if (typeof window !== "undefined" && !path.startsWith("/auth/login")) {
      localStorage.removeItem("forge_token");
      localStorage.removeItem("forge_user");
      window.location.href = "/login";
    }
    throw new ApiError(json.code, json.message || "登录已过期，请重新登录");
  }

  if (json.code !== 0) {
    throw new ApiError(json.code, json.message);
  }

  return json.data;
}

export const api = {
  get: <T>(path: string) => request<T>(path),
  post: <T>(path: string, body?: unknown) =>
    request<T>(path, { method: "POST", body: body ? JSON.stringify(body) : undefined }),
  put: <T>(path: string, body?: unknown) =>
    request<T>(path, { method: "PUT", body: body ? JSON.stringify(body) : undefined }),
  delete: <T>(path: string) => request<T>(path, { method: "DELETE" }),
};

export { ApiError };
