export let csrf = "";
export function setCSRF(value: string) {
  csrf = value;
}
export async function api<T = any>(
  path: string,
  method = "GET",
  body?: unknown,
): Promise<T> {
  const res = await fetch("/api/v1/admin" + path, {
    method,
    credentials: "same-origin",
    cache: "no-store",
    headers: { "Content-Type": "application/json", "X-CSRF-Token": csrf },
    body: body === undefined ? undefined : JSON.stringify(body),
  });
  const data = await res.json();
  if (!res.ok) {
    if (res.status === 401 && path !== "/login" && path !== "/session")
      window.location.replace("/login");
    throw new Error(data.error || "Request failed");
  }
  return data;
}
export type Device = {
  id: number;
  device_uuid: string;
  rustdesk_id: string;
  hostname: string;
  os: string;
  os_version: string;
  arch: string;
  rustdesk_version: string;
  managed_client_version: string;
  status: string;
  first_seen_at: number;
  last_seen_at: number;
  applied_policy_version: number;
  applied_password_version: number;
  password_version: number;
  online: boolean;
  password_synced: boolean;
};
