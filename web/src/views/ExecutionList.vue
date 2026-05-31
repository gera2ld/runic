<script setup lang="ts">
import { ref, computed, onMounted, watch } from "vue";
import { useRoute } from "vue-router";
import AppLayout from "../components/AppLayout.vue";
import Pagination from "../components/Pagination.vue";
import ExecutionTable from "../components/ExecutionTable.vue";
import { fetchHistory } from "../api";
import { useActionPoller } from "../poller";
import { patchHistory, PER_PAGE } from "../utils";
import type { HistoryEntry } from "../utils";

const route = useRoute();
const history = ref<HistoryEntry[]>([]);
const loading = ref(false);
const initialLoading = ref(true);
const p = ref(1);
const perPage = PER_PAGE;
const isSystem = computed(() => route.query?.system === "true");

const paged = computed(() => {
  const start = (p.value - 1) * perPage;
  return history.value.slice(start, start + perPage);
});

async function refresh() {
  loading.value = true;
  try {
    const h = await fetchHistory({ system: isSystem.value });
    history.value = Array.isArray(h) ? h : [];
  } catch (e) {
    console.error(e);
    history.value = [];
  }
  loading.value = false;
  initialLoading.value = false;
  p.value = 1;
}

useActionPoller(
  () => history.value.filter((e) => e.status === "RUNNING").map((e) => e.id),
  (entries) => {
    history.value = patchHistory(history.value, entries);
    if (p.value > Math.max(1, Math.ceil(history.value.length / perPage))) p.value = 1;
  },
);

onMounted(() => refresh());
watch(
  () => route.query.system,
  () => refresh(),
);
</script>

<template>
  <AppLayout>
    <div class="flex items-center justify-between mb-6">
      <h1 class="text-2xl font-semibold">{{ isSystem ? "System Executions" : "Executions" }}</h1>
      <button
        @click="refresh"
        class="px-3 py-1.5 bg-primary-solid hover:bg-primary-hover rounded-lg text-sm font-medium transition"
      >
        Refresh
      </button>
    </div>
    <div v-if="initialLoading" class="text-faint text-sm">Loading...</div>
    <div
      v-else-if="history.length === 0"
      class="text-faint text-sm bg-surface rounded-lg border border-line px-4 py-8 text-center"
    >
      No executions yet.
    </div>
    <template v-else>
      <Pagination v-model="p" :total="history.length" :pageSize="perPage" />
      <ExecutionTable :entries="paged" showDuration />
    </template>
  </AppLayout>
</template>
