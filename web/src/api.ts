import type { ActionDef, HistoryEntry } from "./utils";

async function fetchJSON<T>(url: string): Promise<T> {
  const res = await fetch(url);
  if (!res.ok) throw new Error(`${res.status} ${res.statusText}`);
  return res.json();
}

export function fetchHistory(opts?: {
  system?: boolean;
  historyIds?: number[];
  actionId?: string;
}): Promise<HistoryEntry[]> {
  const url = new URL("/api/history", location.origin);
  url.searchParams.set("system", opts?.system ? "true" : "false");
  if (opts?.historyIds && opts.historyIds.length > 0)
    url.searchParams.set("history_ids", opts.historyIds.join(","));
  if (opts?.actionId) url.searchParams.set("action_id", opts.actionId);
  return fetchJSON(url.pathname + url.search);
}

export function fetchActions(): Promise<ActionDef[]> {
  return fetchJSON("/api/actions");
}

export function fetchActionDetail(id: string): Promise<ActionDef> {
  return fetchJSON(`/api/actions/${encodeURIComponent(id)}`);
}

export function triggerAction(id: string): Promise<Response> {
  return fetch(`/api/actions/${encodeURIComponent(id)}/trigger`, { method: "POST" });
}

export function reloadConfig(): Promise<Response> {
  return fetch(`/api/reload`, { method: "POST" });
}

export function fetchLogs(hid: number): Promise<Response> {
  return fetch(`/api/logs/${hid}`);
}

export function fetchSystem(): Promise<Record<string, unknown>> {
  return fetchJSON("/api/system");
}
