<script setup lang="ts">
import { ref, computed, onMounted, watch } from "vue";
import AppLayout from "../components/AppLayout.vue";
import Breadcrumbs from "../components/Breadcrumbs.vue";
import Pagination from "../components/Pagination.vue";
import ExecutionTable from "../components/ExecutionTable.vue";
import { fetchActionDetail, fetchHistory } from "../api";
import { useActionPoller } from "../poller";
import { patchHistory } from "../utils";
import { actionUrl } from "../urls";
import type { HistoryEntry, ActionDef } from "../utils";

const props = withDefaults(
  defineProps<{
    actionId?: string;
    system?: boolean;
  }>(),
  { actionId: undefined, system: false },
);

const actionDef = ref<ActionDef | null>(null);
const history = ref<HistoryEntry[]>([]);
const loading = ref(true);
const p = ref(1);
const pageSize = 12;

const isScoped = computed(() => !!props.actionId);

const paged = computed(() => {
  const start = (p.value - 1) * pageSize;
  return history.value.slice(start, start + pageSize);
});

const breadcrumbs = computed(() => {
  if (props.actionId) {
    return [
      { label: "Actions", to: "/actions" },
      { label: actionDef.value?.name || props.actionId, to: actionUrl(props.actionId) },
      { label: "Executions" },
    ];
  }
  if (props.system) {
    return [
      { label: "Executions", to: "/executions" },
      { label: "System" },
    ];
  }
  return [{ label: "Executions" }];
});

async function refresh() {
  loading.value = true;
  try {
    if (props.actionId) {
      actionDef.value = null;
      const a = await fetchActionDetail(props.actionId);
      if (a && a.id) actionDef.value = a;
      const h = await fetchHistory({ actionId: props.actionId });
      history.value = Array.isArray(h) ? h : [];
    } else {
      const h = await fetchHistory({ system: props.system });
      history.value = Array.isArray(h) ? h : [];
    }
  } catch (e) {
    console.error(e);
    history.value = [];
  }
  loading.value = false;
  p.value = 1;
}

useActionPoller(
  () => history.value.filter((e) => e.status === "RUNNING").map((e) => e.id),
  (entries) => {
    history.value = patchHistory(history.value, entries);
    if (p.value > Math.max(1, Math.ceil(history.value.length / pageSize))) p.value = 1;
  },
);

onMounted(() => refresh());

watch(
  () => [props.actionId, props.system],
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
      <div class="flex items-center justify-between mb-4">
        <Breadcrumbs :items="breadcrumbs" />
        <button
          @click="refresh"
          class="px-3 py-1.5 bg-primary-solid hover:bg-primary-hover rounded-lg text-sm font-medium transition"
        >
          Refresh
        </button>
      </div>
      <div
        v-if="history.length === 0"
        class="text-faint text-sm bg-surface rounded-lg border border-line px-4 py-8 text-center"
      >
        No executions yet.
      </div>
      <template v-else>
        <Pagination v-model="p" :total="history.length" :pageSize="pageSize" />
        <ExecutionTable :entries="paged" :show-action="!isScoped" show-duration />
      </template>
    </template>
  </AppLayout>
</template>
