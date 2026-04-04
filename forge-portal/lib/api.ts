const BASE_URL = "/api";

interface ApiResult<T> {
  code: number;
  message: string;
  data: T;
}

class ApiError extends Error {
  constructor(
    public code: number,
    message: string,
    public httpStatus?: number
  ) {
    super(message);
  }

  get isAuth() {
    return this.code >= 2000 && this.code < 3000;
  }

  get isValidation() {
    return this.code >= 1000 && this.code < 2000;
  }

  get isNotFound() {
    return this.code >= 4000 && this.code < 5000;
  }
}

async function request<T>(
  path: string,
  options: RequestInit & { timeout?: number } = {}
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

  const { timeout, ...fetchOptions } = options;
  const controller = new AbortController();
  const timeoutId = timeout
    ? setTimeout(() => controller.abort(), timeout)
    : undefined;

  let res: Response;
  try {
    res = await fetch(`${BASE_URL}${path}`, {
      ...fetchOptions,
      headers,
      signal: controller.signal,
    });
  } catch (err) {
    if (timeoutId) clearTimeout(timeoutId);
    if (err instanceof DOMException && err.name === "AbortError") {
      throw new ApiError(-2, "请求超时，AI 分析可能需要更长时间。请刷新页面查看对话历史。");
    }
    throw err;
  } finally {
    if (timeoutId) clearTimeout(timeoutId);
  }

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
    throw new ApiError(json.code, json.message || "登录已过期，请重新登录", res.status);
  }

  if (json.code !== 0) {
    throw new ApiError(json.code, json.message, res.status);
  }

  return json.data;
}

export const api = {
  get: <T>(path: string) => request<T>(path),
  post: <T>(path: string, body?: unknown, opts?: { timeout?: number }) =>
    request<T>(path, { method: "POST", body: body ? JSON.stringify(body) : undefined, ...opts }),
  put: <T>(path: string, body?: unknown) =>
    request<T>(path, { method: "PUT", body: body ? JSON.stringify(body) : undefined }),
  delete: <T>(path: string) => request<T>(path, { method: "DELETE" }),
};

export { ApiError };
