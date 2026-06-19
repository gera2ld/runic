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

export function fetchActions(system?: boolean): Promise<ActionDef[]> {
  const url = new URL("/api/actions", location.origin);
  if (system) url.searchParams.set("system", "true");
  return fetchJSON(url.pathname + url.search);
}

export function fetchActionDetail(id: string): Promise<ActionDef> {
  return fetchJSON(`/api/actions/${encodeURIComponent(id)}`);
}

export function triggerAction(id: string): Promise<Response> {
  return fetch(`/api/actions/${encodeURIComponent(id)}/trigger`, { method: "POST" });
}

export function fetchLogs(hid: number): Promise<Response> {
  return fetch(`/api/logs/${hid}`);
}

export function fetchSystem(): Promise<Record<string, unknown>> {
  return fetchJSON("/api/system");
}

type Unsubscribe = () => void;

interface PollSubscriber {
  getIds: () => number[];
  onUpdate: (entries: HistoryEntry[]) => void;
}

function createActionPoller() {
  const subscribers = new Map<number, PollSubscriber>();
  let nextId = 0;
  let pollTimer: ReturnType<typeof setInterval> | null = null;

  function pollAll() {
    const allIds = new Set<number>();
    for (const entry of subscribers.values()) {
      const ids = entry.getIds();
      if (ids) {
        for (const id of ids) allIds.add(id);
      }
    }
    if (allIds.size === 0) return;
    fetchHistory({ historyIds: Array.from(allIds) })
      .then((entries) => {
        for (const entry of subscribers.values()) {
          entry.onUpdate(entries);
        }
      })
      .catch((e) => console.error(e));
  }

  function startPolling() {
    if (pollTimer) return;
    pollAll();
    pollTimer = setInterval(pollAll, 3000);
  }

  function stopPollingIfEmpty() {
    if (subscribers.size === 0 && pollTimer) {
      clearInterval(pollTimer);
      pollTimer = null;
    }
  }

  return {
    subscribe(getIds: () => number[], onUpdate: (entries: HistoryEntry[]) => void): Unsubscribe {
      const id = nextId++;
      subscribers.set(id, { getIds, onUpdate });
      startPolling();
      return () => {
        subscribers.delete(id);
        stopPollingIfEmpty();
      };
    },
  };
}

export const actionPoller = createActionPoller();
