<script setup lang="ts">
import { ref, computed, onMounted, watch } from "vue";
import { useRoute } from "vue-router";
import AppLayout from "../components/AppLayout.vue";
import Breadcrumbs from "../components/Breadcrumbs.vue";
import { fetchActionDetail, fetchHistory, triggerAction } from "../api";
import { useActionPoller } from "../poller";
import { patchHistory, statusClass, formatDate, timeAgo, formatTimeout, PER_PAGE } from "../utils";
import { actionUrl, actionExecutionsUrl, executionUrl } from "../urls";
import type { HistoryEntry, ActionDef } from "../utils";

const route = useRoute();
const actionDef = ref<ActionDef | null>(null);
const history = ref<HistoryEntry[]>([]);
const loading = ref(true);
const p = ref(1);
const perPage = PER_PAGE;

const filtered = computed(() => {
  const h = history.value;
  if (!Array.isArray(h)) return [];
  const aid = (route.params.id as string) || "";
  return h.filter((e) => e.action_id === aid);
});

const tp = computed(() => Math.max(1, Math.ceil(filtered.value.length / perPage)));

const lastRun = computed(() => {
  const h = filtered.value;
  return Array.isArray(h) && h.length > 0 ? timeAgo(h[0].created_at) : "-";
});
const lastRunStatus = computed(() => {
  const h = filtered.value;
  return Array.isArray(h) && h.length > 0 ? h[0].status : "";
});

async function refresh() {
  try {
    actionDef.value = null;
    const a = await fetchActionDetail(route.params.id as string);
    if (a && a.id) actionDef.value = a;
    const h = await fetchHistory({ actionId: route.params.id as string });
    history.value = Array.isArray(h) ? h : [];
    if (p.value > tp.value) p.value = 1;
  } catch (e) {
    console.error(e);
    history.value = [];
  }
  loading.value = false;
}

useActionPoller(
  () => filtered.value.filter((e) => e.status === "RUNNING").map((e) => e.id),
  (entries) => {
    history.value = patchHistory(history.value, entries);
  },
);

async function trigger() {
  try {
    const res = await triggerAction(route.params.id as string);
    if (!res.ok) throw new Error(await res.text());
    refresh();
  } catch (e) {
    console.error(e);
  }
}

const breadcrumbs = computed(() => [
  { label: "Actions", to: "/actions" },
  {
    label: actionDef.value?.name || (route.params.id as string),
    to: actionUrl(route.params.id as string),
  },
]);

onMounted(() => refresh());
watch(
  () => route.params.id,
  () => {
    p.value = 1;
    loading.value = true;
    refresh();
  },
);
</script>

<template>
  <AppLayout>
    <div v-if="loading" class="text-faint text-sm">Loading...</div>
    <template v-else>
      <Breadcrumbs :items="breadcrumbs" class="mb-4" />
      <div v-if="actionDef">
        <div class="grid grid-cols-1 lg:grid-cols-2 gap-6 mb-8">
          <div class="bg-surface border border-line rounded-lg p-6 flex flex-col">
            <div class="flex items-center justify-between mb-4">
              <div class="flex items-center gap-3 min-w-0">
                <h1 class="text-xl font-semibold text-primary truncate">{{ actionDef.name }}</h1>
                <div class="flex flex-wrap gap-1.5">
                  <span
                    class="text-[10px] px-1.5 py-0.5 bg-subtle text-body rounded font-mono"
                    title="Timeout"
                    >{{ formatTimeout(actionDef.timeout) }}</span
                  >
                  <span
                    class="text-[10px] px-1.5 py-0.5 bg-subtle text-body rounded font-mono"
                    :title="
                      'Concurrency: ' +
                      (actionDef.concurrency === 0 ? 'unlimited' : actionDef.concurrency)
                    "
                  >
                    {{ actionDef.concurrency === 0 ? "\u221e" : "x" + actionDef.concurrency }}
                  </span>
                  <span
                    v-if="actionDef.cron"
                    class="text-[10px] px-1.5 py-0.5 bg-accent/10 text-accent-dim rounded font-mono"
                    :title="'Cron: ' + actionDef.cron"
                    >Cron</span
                  >
                </div>
              </div>
              <button
                @click="trigger"
                class="px-4 py-2 bg-primary-solid hover:bg-primary-hover rounded-lg text-sm font-medium transition"
              >
                Run
              </button>
            </div>
            <div class="space-y-2 text-sm font-mono text-muted mb-6 *:flex *:items-center *:gap-2">
              <div><span class="text-faint">ID:</span> {{ actionDef.id }}</div>
              <div v-if="actionDef.cron">
                <span class="text-faint">Cron:</span>
                <span class="text-accent">{{ actionDef.cron }}</span>
              </div>
              <div v-if="actionDef.next_run">
                <span class="text-faint">Next run:</span>
                <span class="text-subdued">{{ formatDate(actionDef.next_run) }}</span>
              </div>
              <div>
                <span class="text-faint">Last run:</span>
                <span class="text-subdued">{{ lastRun }}</span>
                <span
                  v-if="lastRunStatus"
                  :class="statusClass(lastRunStatus)"
                  class="font-medium text-xs"
                  >{{ lastRunStatus }}</span
                >
              </div>
              <div><span class="text-faint">CWD:</span> {{ actionDef.cwd }}</div>
            </div>
          </div>

          <div>
            <div class="flex items-center justify-between mb-3">
              <h2 class="text-lg font-semibold text-subdued">Recent Executions</h2>
              <router-link
                :to="actionExecutionsUrl(actionDef.id)"
                class="text-xs text-primary hover:text-primary"
                >View all &rarr;</router-link
              >
            </div>
            <div
              v-if="filtered.length === 0"
              class="text-faint text-sm bg-surface rounded-lg border border-line px-4 py-8 text-center"
            >
              No executions yet.
            </div>
            <div v-else class="overflow-x-auto rounded-lg border border-line">
              <table class="w-full text-sm">
                <thead class="bg-surface text-muted uppercase text-xs">
                  <tr>
                    <th class="px-4 py-2 text-left w-16">ID</th>
                    <th class="px-4 py-2 text-left w-24">Status</th>
                    <th class="px-4 py-2 text-left w-48">Started</th>
                    <th class="px-4 py-2 text-left w-20">Log</th>
                  </tr>
                </thead>
                <tbody class="divide-y divide-rule">
                  <tr
                    v-for="e in filtered.slice(0, 5)"
                    :key="e.id"
                    class="hover:bg-surface/50 transition"
                  >
                    <td class="px-4 py-2 font-mono text-faint">{{ e.id }}</td>
                    <td class="px-4 py-2">
                      <span
                        :class="statusClass(e.status)"
                        class="px-2 py-0.5 rounded text-xs font-bold text-body"
                        >{{ e.status }}</span
                      >
                    </td>
                    <td class="px-4 py-2 text-dim text-xs" :title="formatDate(e.created_at)">
                      {{ timeAgo(e.created_at) }}
                    </td>
                    <td class="px-4 py-2">
                      <router-link
                        :to="executionUrl(e.action_id, e.id)"
                        class="text-accent hover:text-accent-dim text-xs font-medium"
                        >View</router-link
                      >
                    </td>
                  </tr>
                </tbody>
              </table>
            </div>
          </div>
        </div>

        <div>
          <h2 class="text-lg font-semibold mb-3 text-subdued">Script</h2>
          <pre
            class="p-4 bg-surface border border-line rounded-lg text-sm text-subdued whitespace-pre-wrap font-mono"
            >{{ actionDef.command }}</pre
          >
        </div>
      </div>
      <div v-else class="text-faint mb-8">Action not found.</div>
    </template>
  </AppLayout>
</template>
