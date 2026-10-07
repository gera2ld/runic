<script setup lang="ts">
import { ref, onMounted } from "vue";
import AppLayout from "../components/AppLayout.vue";
import ExecutionTable from "../components/ExecutionTable.vue";
import { fetchHistory, fetchSystem, reloadConfig } from "../api";
import { useActionPoller } from "../poller";
import { patchHistory } from "../utils";
import type { HistoryEntry } from "../utils";

interface ReloadResult {
  status: string;
  added: string[];
  removed: string[];
  updated: string[];
  backfilled: string[];
  warning?: string;
}

interface SystemData {
  version: string;
  os: string;
  arch: string;
  uptime: string;
  goroutines: number;
  cpus: number;
  config: Record<string, string>;
  environment: { name: string; value: string }[];
}

const data = ref<SystemData>({
  version: "",
  os: "",
  arch: "",
  uptime: "",
  goroutines: 0,
  cpus: 0,
  config: {},
  environment: [],
});
const history = ref<HistoryEntry[]>([]);
const loading = ref(true);
const reloading = ref(false);
const reloadMsg = ref<{ ok: boolean; text: string } | null>(null);

async function refresh() {
  loading.value = true;
  try {
    data.value = (await fetchSystem()) as unknown as SystemData;
    const h = await fetchHistory({ system: true });
    history.value = Array.isArray(h) ? h : [];
  } catch (e) {
    console.error(e);
  }
  loading.value = false;
}

useActionPoller(
  () => history.value.filter((e) => e.status === "RUNNING").map((e) => e.id),
  (entries) => {
    history.value = patchHistory(history.value, entries);
  },
);

function summarizeReload(r: ReloadResult): string {
  const parts: string[] = [];
  if (r.added?.length) parts.push(`added ${r.added.join(", ")}`);
  if (r.removed?.length) parts.push(`removed ${r.removed.join(", ")}`);
  if (r.updated?.length) parts.push(`updated ${r.updated.join(", ")}`);
  if (r.backfilled?.length) parts.push(`backfilled ${r.backfilled.join(", ")}`);
  if (r.warning) parts.push(`warning: ${r.warning}`);
  return parts.length ? `Reloaded: ${parts.join("; ")}.` : "Reloaded, no changes.";
}

async function reload() {
  if (reloading.value) return;
  reloading.value = true;
  reloadMsg.value = null;
  try {
    const res = await reloadConfig();
    const text = await res.text();
    if (!res.ok) {
      reloadMsg.value = { ok: false, text: `Reload failed (${res.status}): ${text}` };
    } else {
      reloadMsg.value = { ok: true, text: summarizeReload(JSON.parse(text) as ReloadResult) };
      refresh();
    }
  } catch (e) {
    reloadMsg.value = { ok: false, text: `Reload failed: ${e}` };
  }
  reloading.value = false;
}

onMounted(() => refresh());
</script>

<template>
  <AppLayout>
    <div class="flex items-center justify-between mb-2">
      <h1 class="text-2xl font-semibold">System</h1>
      <div class="flex items-center gap-3">
        <button
          @click="reload"
          :disabled="reloading"
          class="px-3 py-1.5 bg-primary/20 hover:bg-primary/30 border border-primary/30 rounded-lg text-sm text-primary font-medium transition disabled:opacity-50"
        >
          {{ reloading ? "Reloading…" : "Reload Config" }}
        </button>
        <button
          @click="refresh"
          class="px-3 py-1.5 bg-primary-solid hover:bg-primary-hover rounded-lg text-sm font-medium transition"
        >
          Refresh
        </button>
      </div>
    </div>

    <div
      v-if="reloadMsg"
      class="text-sm rounded-lg border px-4 py-2 mb-4"
      :class="
        reloadMsg.ok
          ? 'bg-surface border-line text-subdued'
          : 'bg-status-failed/10 border-status-failed/30 text-status-failed'
      "
    >
      {{ reloadMsg.text }}
    </div>

    <div v-if="loading" class="text-faint text-sm py-4">Loading...</div>
    <template v-else>
      <div class="flex flex-wrap gap-x-6 gap-y-2 text-xs text-muted mb-8 pb-4 border-b border-line">
        <div>
          <span class="text-faint">Version:</span>
          <span class="font-mono text-subdued ml-1">{{ data.version }}</span>
        </div>
        <div>
          <span class="text-faint">OS:</span>
          <span class="text-subdued ml-1">{{ data.os }}/{{ data.arch }}</span>
        </div>
        <div>
          <span class="text-faint">Uptime:</span>
          <span class="text-subdued ml-1">{{ data.uptime }}</span>
        </div>
        <div>
          <span class="text-faint">Goroutines:</span>
          <span class="text-subdued ml-1">{{ data.goroutines }}</span>
        </div>
        <div>
          <span class="text-faint">CPUs:</span>
          <span class="text-subdued ml-1">{{ data.cpus }}</span>
        </div>
      </div>

      <div class="grid grid-cols-1 lg:grid-cols-3 gap-8 mb-10">
        <div class="lg:col-span-1 @container">
          <h2 class="text-lg font-semibold mb-3 text-subdued">Configuration</h2>
          <div
            class="bg-surface border border-line rounded-lg p-4 grid grid-cols-1 @md:grid-cols-2 gap-x-6 gap-y-2 text-sm"
          >
            <div v-for="(v, k) in data.config" :key="k" class="flex justify-between">
              <span class="text-dim">{{ k }}</span>
              <span class="font-mono text-subdued">{{ v }}</span>
            </div>
          </div>
        </div>

        <div class="lg:col-span-2">
          <div class="flex items-center justify-between mb-3">
            <h2 class="text-lg font-semibold text-subdued">Recent Executions</h2>
            <router-link
              to="/executions?system=true"
              class="text-xs text-primary hover:text-primary"
              >View all &rarr;</router-link
            >
          </div>
          <div
            v-if="history.length === 0"
            class="text-faint text-sm bg-surface rounded-lg border border-line px-4 py-8 text-center"
          >
            No system executions yet.
          </div>
          <ExecutionTable v-else :entries="history" :limit="5" />
        </div>
      </div>

      <h2 class="text-lg font-semibold mb-3 text-subdued">Environment Variables</h2>
      <div class="overflow-x-auto rounded-lg border border-line">
        <table class="w-full text-sm">
          <thead class="bg-surface text-muted uppercase text-xs">
            <tr>
              <th class="px-4 py-3 text-left w-1/3">Name</th>
              <th class="px-4 py-3 text-left">Value</th>
            </tr>
          </thead>
          <tbody class="divide-y divide-rule">
            <tr v-for="e in data.environment" :key="e.name" class="hover:bg-surface/50 transition">
              <td class="px-4 py-2 font-mono text-primary">{{ e.name }}</td>
              <td class="px-4 py-2 font-mono text-subdued break-all">{{ e.value }}</td>
            </tr>
          </tbody>
        </table>
      </div>
    </template>
  </AppLayout>
</template>
