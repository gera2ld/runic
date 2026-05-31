<script setup lang="ts">
import { ref, computed, onMounted } from "vue";
import AppLayout from "../components/AppLayout.vue";
import ActionCard from "../components/ActionCard.vue";
import ExecutionTable from "../components/ExecutionTable.vue";
import { fetchHistory, fetchActions } from "../api";
import { useActionPoller } from "../poller";
import { patchHistory } from "../utils";
import type { HistoryEntry, ActionDef } from "../utils";

const history = ref<HistoryEntry[]>([]);
const loading = ref(true);
const actions = ref<ActionDef[]>([]);
const actionLoading = ref(true);
const toast = ref("");

const recentActions = computed(() => {
  const h = history.value;
  if (!Array.isArray(h)) return [];
  const seen = new Set<string>();
  const result: ActionDef[] = [];
  for (const entry of h) {
    const aid = entry.action_id;
    if (!seen.has(aid)) {
      seen.add(aid);
      const found = actions.value.find((a) => a.id === aid);
      if (found) result.push(found);
    }
    if (result.length >= 4) break;
  }
  return result;
});

async function refreshHistory() {
  loading.value = true;
  try {
    const h = await fetchHistory();
    history.value = Array.isArray(h) ? h : [];
  } catch (e) {
    console.error(e);
    history.value = [];
  }
  loading.value = false;
}

async function refreshActions() {
  actionLoading.value = true;
  try {
    const a = await fetchActions();
    actions.value = Array.isArray(a) ? a : [];
  } catch (e) {
    console.error(e);
    actions.value = [];
  }
  actionLoading.value = false;
}

function refresh() {
  refreshHistory();
  refreshActions();
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
  <AppLayout :toast="toast" @toast="(v: string) => (toast = v)">
    <div class="flex items-center justify-between mb-6">
      <h1 class="text-2xl font-semibold">Overview</h1>
      <button
        @click="refresh"
        class="px-3 py-1.5 bg-primary-solid hover:bg-primary-hover rounded-lg text-sm font-medium transition"
      >
        Refresh
      </button>
    </div>

    <div class="grid grid-cols-1 lg:grid-cols-2 gap-6 mb-8">
      <div>
        <div class="flex items-center justify-between mb-3">
          <h2 class="text-lg font-semibold text-subdued">Recent Actions</h2>
          <router-link to="/actions" class="text-xs text-primary hover:text-primary"
            >View all &rarr;</router-link
          >
        </div>
        <div
          v-if="actionLoading"
          class="text-faint text-sm bg-surface rounded-lg border border-line px-4 py-6 text-center"
        >
          Loading...
        </div>
        <div
          v-else-if="recentActions.length === 0"
          class="text-faint text-sm bg-surface rounded-lg border border-line px-4 py-6 text-center"
        >
          No actions yet.
        </div>
        <div v-else class="grid grid-cols-1 sm:grid-cols-2 gap-3">
          <ActionCard
            v-for="a in recentActions"
            :key="a.id"
            :action="a"
            @triggered="refreshActions"
          />
        </div>
      </div>

      <div>
        <div class="flex items-center justify-between mb-3">
          <h2 class="text-lg font-semibold text-subdued">Recent Executions</h2>
          <router-link to="/executions" class="text-xs text-primary hover:text-primary"
            >View all &rarr;</router-link
          >
        </div>
        <div
          v-if="loading"
          class="text-faint text-sm bg-surface rounded-lg border border-line px-4 py-6 text-center"
        >
          Loading...
        </div>
        <div
          v-else-if="history.length === 0"
          class="text-faint text-sm bg-surface rounded-lg border border-line px-4 py-6 text-center"
        >
          No executions yet.
        </div>
        <ExecutionTable v-else :entries="history" :limit="5" />
      </div>
    </div>
  </AppLayout>
</template>
