import { onMounted, onUnmounted } from "vue";
import { actionPoller } from "./api";
import type { HistoryEntry } from "./utils";

export function useActionPoller(
  getIds: () => number[],
  onUpdate: (entries: HistoryEntry[]) => void,
): void {
  let unsubscribe: (() => void) | null = null;
  onMounted(() => {
    unsubscribe = actionPoller.subscribe(getIds, onUpdate);
  });
  onUnmounted(() => {
    if (unsubscribe) unsubscribe();
  });
}
