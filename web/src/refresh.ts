import { ref, onMounted, onUnmounted, type Ref } from "vue";

export type RefreshHandler = () => void | Promise<void>;

const REFRESH_INTERVAL_MS = 5000;
const TICK_MS = 100;
// Keep the dimmed ring visible briefly even when a refresh returns instantly.
const REFRESH_MIN_MS = 300;

const refreshHandler: Ref<RefreshHandler | null> = ref(null);
const refreshing = ref(false);
// 1 = just refreshed (full ring), 0 = depleted (refresh now).
const progress = ref(1);

let timer: ReturnType<typeof setInterval> | null = null;
let lastTick = 0;

function delay(ms: number): Promise<void> {
  return new Promise((resolve) => setTimeout(resolve, ms));
}

function stopTimer(): void {
  if (timer !== null) {
    clearInterval(timer);
    timer = null;
  }
}

function startTimer(): void {
  stopTimer();
  lastTick = Date.now();
  timer = setInterval(() => {
    const now = Date.now();
    const dt = now - lastTick;
    lastTick = now;
    progress.value = Math.max(0, progress.value - dt / REFRESH_INTERVAL_MS);
    if (progress.value <= 0) {
      // Ring emptied: fire the refresh and pause draining until it settles.
      stopTimer();
      void triggerRefresh();
    }
  }, TICK_MS);
}

async function triggerRefresh(): Promise<void> {
  const handler = refreshHandler.value;
  if (!handler || refreshing.value) return;
  refreshing.value = true;
  progress.value = 1;
  try {
    await Promise.all([Promise.resolve(handler()), delay(REFRESH_MIN_MS)]);
  } finally {
    refreshing.value = false;
    if (refreshHandler.value === handler) {
      startTimer();
    }
  }
}

export function usePageRefresh(handler: RefreshHandler): void {
  onMounted(() => {
    refreshHandler.value = handler;
    progress.value = 1;
    startTimer();
  });
  onUnmounted(() => {
    if (refreshHandler.value === handler) {
      refreshHandler.value = null;
    }
    stopTimer();
  });
}

export function useRefreshButton() {
  function onClick(): void {
    stopTimer();
    progress.value = 1;
    void triggerRefresh();
  }
  return { refreshHandler, refreshing, progress, onClick };
}
