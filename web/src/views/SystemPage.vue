<script setup lang="ts">
import { ref, onMounted } from "vue";
import AppLayout from "../components/AppLayout.vue";
import ExecutionTable from "../components/ExecutionTable.vue";
import { fetchHistory, fetchSystem } from "../api";
import { useActionPoller } from "../poller";
import { patchHistory } from "../utils";
import type { HistoryEntry } from "../utils";

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

onMounted(() => refresh());
</script>

<template>
  <AppLayout>
    <div class="flex items-center justify-between mb-2">
      <h1 class="text-2xl font-semibold">System</h1>
      <div class="flex items-center gap-3">
        <router-link to="/actions?system=true" class="text-sm text-primary hover:underline"
          >All System Actions</router-link
        >
        <button
          @click="refresh"
          class="px-3 py-1.5 bg-primary-solid hover:bg-primary-hover rounded-lg text-sm font-medium transition"
        >
          Refresh
        </button>
      </div>
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

      <div class="grid grid-cols-1 lg:grid-cols-2 gap-8 mb-10">
        <div>
          <h2 class="text-lg font-semibold mb-3 text-subdued">Configuration</h2>
          <div class="bg-surface border border-line rounded-lg p-4 space-y-2 text-sm">
            <div v-for="(v, k) in data.config" :key="k" class="flex justify-between">
              <span class="text-dim">{{ k }}</span>
              <span class="font-mono text-subdued">{{ v }}</span>
            </div>
          </div>
        </div>

        <div>
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
