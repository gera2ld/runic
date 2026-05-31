export const MAX_HISTORY = 500;
export const PER_PAGE = 20;
export const MAX_VISIBLE_PAGES = 5;

export function statusClass(s: string): string {
  return (
    (
      {
        RUNNING: "text-status-running",
        SUCCESS: "text-status-success",
        FAILED: "text-status-failed",
        TIMEOUT: "text-status-timeout",
      } as Record<string, string>
    )[s] || "text-muted"
  );
}

export function formatMs(ms: number | null | undefined): string {
  if (!ms && ms !== 0) return "-";
  return ms < 1000 ? ms + "ms" : (ms / 1000).toFixed(1) + "s";
}

export function formatDate(ts: string | null | undefined): string {
  return ts ? new Date(ts).toLocaleString() : "-";
}

export function timeAgo(ts: string | null | undefined): string {
  if (!ts) return "-";
  const diff = Date.now() - new Date(ts).getTime();
  if (diff < 0) return "just now";
  const sec = Math.floor(diff / 1000);
  if (sec < 60) return sec + "s ago";
  const min = Math.floor(sec / 60);
  if (min < 60) return min + "m ago";
  const hr = Math.floor(min / 60);
  if (hr < 24) return hr + "h ago";
  const day = Math.floor(hr / 24);
  if (day < 30) return day + "d ago";
  return formatDate(ts);
}

export function formatTimeout(seconds: number): string {
  return seconds < 60 ? seconds + "s" : Math.floor(seconds / 60) + "m";
}

export interface HistoryEntry {
  id: number;
  action_id: string;
  status: string;
  duration_ms: number | null;
  created_at: string;
  log_file_path?: string;
}

export interface ActionDef {
  id: string;
  name: string;
  timeout: number;
  command: string;
  cwd: string;
  cron: string | null;
  concurrency: number;
  next_run: string | null;
  last_run: string | null;
  last_run_status: string | null;
}

export function patchHistory(current: HistoryEntry[], updates: HistoryEntry[]): HistoryEntry[] {
  if (!Array.isArray(updates)) return current;
  const res = [...current];
  for (const u of updates) {
    const idx = res.findIndex((e) => e.id === u.id);
    if (idx !== -1) {
      res[idx] = { ...res[idx], ...u };
    } else {
      res.push(u);
    }
  }
  res.sort((a, b) => b.id - a.id);
  return res.slice(0, MAX_HISTORY);
}
