<script setup lang="ts">
import { ref, computed, onMounted } from "vue";
import { useRoute } from "vue-router";
import AppLayout from "../components/AppLayout.vue";
import Breadcrumbs from "../components/Breadcrumbs.vue";
import { fetchHistory, fetchLogs } from "../api";
import { statusClass, formatMs, formatDate, timeAgo } from "../utils";
import { actionUrl, actionExecutionsUrl } from "../urls";
import { usePageRefresh } from "../refresh";
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

usePageRefresh(refresh);
onMounted(() => refresh());
</script>

<template>
  <AppLayout>
    <div v-if="loading" class="text-faint">Loading...</div>
    <template v-else>
      <Breadcrumbs :items="breadcrumbs" class="mb-4" />
      <div v-if="entry" class="mb-4 sm:mb-6 flex items-center gap-x-4 gap-y-2 flex-wrap">
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
        class="bg-surface border border-line rounded-lg p-3 sm:p-4 font-mono text-xs text-subdued leading-relaxed overflow-auto max-h-[70vh]"
        >{{ logContent || "No log content." }}</pre
      >
    </template>
  </AppLayout>
</template>
