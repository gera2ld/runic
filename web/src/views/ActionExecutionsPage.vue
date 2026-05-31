<script setup lang="ts">
import { ref, computed, onMounted, watch } from "vue";
import { useRoute, useRouter } from "vue-router";
import AppLayout from "../components/AppLayout.vue";
import Breadcrumbs from "../components/Breadcrumbs.vue";
import Pagination from "../components/Pagination.vue";
import ExecutionTable from "../components/ExecutionTable.vue";
import { fetchActionDetail, fetchHistory } from "../api";
import { useActionPoller } from "../poller";
import { patchHistory } from "../utils";
import { actionUrl } from "../urls";
import type { HistoryEntry, ActionDef } from "../utils";

const route = useRoute();
const router = useRouter();
const actionDef = ref<ActionDef | null>(null);
const history = ref<HistoryEntry[]>([]);
const loading = ref(true);
const p = ref(1);
const pageSizeOptions = [15, 20, 50, 100];
const pageSize = ref(15);

function syncPageSize() {
  const raw = route.query?.page_size ? parseInt(route.query.page_size as string, 10) : 0;
  pageSize.value = pageSizeOptions.includes(raw) ? raw : 15;
}

const filtered = computed(() => {
  const h = history.value;
  if (!Array.isArray(h)) return [];
  const aid = (route.params.id as string) || "";
  return h.filter((e) => e.action_id === aid);
});
const paged = computed(() => {
  const start = (p.value - 1) * pageSize.value;
  return filtered.value.slice(start, start + pageSize.value);
});

const breadcrumbs = computed(() => [
  { label: "Actions", to: "/actions" },
  {
    label: actionDef.value?.name || (route.params.id as string),
    to: actionUrl(route.params.id as string),
  },
  { label: "Executions" },
]);

function onPageSizeChange(newSize: number) {
  pageSize.value = newSize;
  p.value = 1;
  router.replace({
    path: route.path,
    query: { ...route.query, page_size: String(newSize) },
  });
}

async function refresh() {
  try {
    actionDef.value = null;
    const a = await fetchActionDetail(route.params.id as string);
    if (a && a.id) actionDef.value = a;
    const h = await fetchHistory({ actionId: route.params.id as string });
    history.value = Array.isArray(h) ? h : [];
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

onMounted(() => {
  syncPageSize();
  refresh();
});
watch(
  () => route.query.page_size,
  () => {
    syncPageSize();
    p.value = 1;
  },
  { immediate: true },
);
watch(
  () => route.params.id,
  () => {
    p.value = 1;
    loading.value = true;
    syncPageSize();
    refresh();
  },
);
</script>

<template>
  <AppLayout>
    <div v-if="loading" class="text-faint text-sm">Loading...</div>
    <template v-else>
      <Breadcrumbs :items="breadcrumbs" class="mb-4" />
      <div
        v-if="filtered.length === 0"
        class="text-faint text-sm bg-surface rounded-lg border border-line px-4 py-8 text-center"
      >
        No executions yet.
      </div>
      <template v-else>
        <Pagination
          v-model="p"
          :total="filtered.length"
          :pageSize="pageSize"
          :pageSizeOptions="pageSizeOptions"
          @update:pageSize="onPageSizeChange"
        />
        <ExecutionTable :entries="paged" :showAction="false" showDuration />
      </template>
    </template>
  </AppLayout>
</template>
