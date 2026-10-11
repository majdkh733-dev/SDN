const base = import.meta.env.VITE_API_BASE_URL ?? "";

export class ApiError extends Error {
  code: string;
  status: number;
  constructor(message: string, code: string, status: number) {
    super(message);
    this.code = code;
    this.status = status;
  }
}

export async function api<T>(path: string, init: RequestInit = {}): Promise<T> {
  const headers = new Headers(init.headers);
  if (init.body && !headers.has("Content-Type")) headers.set("Content-Type", "application/json");
  const res = await fetch(base + path, { ...init, headers, credentials: "include" });
  const body = (await res.json().catch(() => ({}))) as {
    data?: T;
    error?: { code?: string; message?: string };
  };
  if (!res.ok) {
    throw new ApiError(body.error?.message || "Request failed", body.error?.code || "error", res.status);
  }
  return body.data as T;
}

export type Principal = {
  id: string;
  email: string;
  display_name: string;
  role: string;
  organization: { organization_id: string; organization_name: string; organization_slug: string; role: string };
  memberships: { organization_id: string; organization_name: string; organization_slug: string; role: string }[];
};

export function can(role: string, action: "read" | "operate" | "author" | "approve" | "admin" | "restore") {
  const map: Record<string, string[]> = {
    read: ["viewer", "operator", "engineer", "approver", "administrator", "recovery_operator"],
    operate: ["operator", "engineer", "administrator"],
    author: ["engineer", "administrator"],
    approve: ["approver", "administrator"],
    admin: ["administrator"],
    restore: ["recovery_operator", "administrator"],
  };
  return map[action].includes(role);
}
