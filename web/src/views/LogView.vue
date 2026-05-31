<script setup lang="ts">
import { ref, computed, onMounted } from "vue";
import { useRoute } from "vue-router";
import AppLayout from "../components/AppLayout.vue";
import Breadcrumbs from "../components/Breadcrumbs.vue";
import { fetchHistory, fetchLogs } from "../api";
import { useActionPoller } from "../poller";
import { statusClass, formatMs, formatDate, timeAgo } from "../utils";
import { actionUrl, actionExecutionsUrl } from "../urls";
import type { HistoryEntry } from "../utils";

const route = useRoute();
const entry = ref<HistoryEntry | null>(null);
const logContent = ref("");
const loading = ref(true);

const breadcrumbs = computed(() => {
  const actionID = entry.value?.action_id || (route.params.id as string) || "";
  const hid = (route.params.hid as string) || "";
  return [
    { label: "Actions", to: "/actions" },
    { label: actionID, to: actionUrl(actionID) },
    { label: "Executions", to: actionExecutionsUrl(actionID) },
    { label: "#" + hid },
  ];
});

async function refresh() {
  try {
    const hid = route.params.hid ? parseInt(route.params.hid as string) : 0;
    const h = await fetchHistory({ historyIds: [hid] });
    if (Array.isArray(h) && h.length > 0) {
      entry.value = h[0];
    }
    const lres = await fetchLogs(hid);
    if (lres.ok) logContent.value = await lres.text();
  } catch (e) {
    console.error(e);
  }
  loading.value = false;
}

useActionPoller(
  () => {
    if (entry.value && entry.value.status === "RUNNING") return [entry.value.id];
    return [];
  },
  (entries) => {
    if (entry.value && entries.length > 0) {
      for (const e of entries) {
        if (e.id === entry.value.id) {
          entry.value = { ...entry.value, ...e };
          break;
        }
      }
    }
    const hid = route.params.hid ? parseInt(route.params.hid as string) : 0;
    fetchLogs(hid)
      .then((r) => (r.ok ? r.text() : null))
      .then((t) => {
        if (t) logContent.value = t;
      })
      .catch(() => {});
  },
);

onMounted(() => refresh());
</script>

<template>
  <AppLayout>
    <div v-if="loading" class="text-faint">Loading...</div>
    <template v-else>
      <Breadcrumbs :items="breadcrumbs" class="mb-4" />
      <div v-if="entry" class="mb-6 flex items-center gap-4 flex-wrap">
        <span class="text-dim text-sm">
          Action:
          <router-link
            :to="actionUrl(entry.action_id)"
            class="text-primary font-mono hover:underline"
            >{{ entry.action_id }}</router-link
          >
        </span>
        <span class="text-dim text-sm"
          >Duration: <span class="text-subdued">{{ formatMs(entry.duration_ms) }}</span></span
        >
        <span class="text-dim text-sm"
          >Started:
          <span class="text-subdued" :title="formatDate(entry.created_at)">{{
            timeAgo(entry.created_at)
          }}</span></span
        >
        <span
          :class="statusClass(entry.status)"
          class="py-1.5 rounded text-sm font-bold text-body"
          >{{ entry.status }}</span
        >
      </div>
      <pre
        class="bg-surface border border-line rounded-lg p-4 font-mono text-xs text-subdued leading-relaxed overflow-auto max-h-[70vh]"
        >{{ logContent || "No log content." }}</pre
      >
    </template>
  </AppLayout>
</template>
