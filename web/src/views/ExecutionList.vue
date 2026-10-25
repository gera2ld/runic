<script setup lang="ts">
import { ref, computed, onMounted, watch } from "vue";
import AppLayout from "../components/AppLayout.vue";
import Breadcrumbs from "../components/Breadcrumbs.vue";
import Pagination from "../components/Pagination.vue";
import ExecutionTable from "../components/ExecutionTable.vue";
import { fetchActionDetail, fetchHistory } from "../api";
import { actionUrl } from "../urls";
import { usePageRefresh } from "../refresh";
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
    return [{ label: "Executions", to: "/executions" }, { label: "System" }];
  }
  return [{ label: "Executions" }];
});

async function refresh() {
  try {
    if (props.actionId) {
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
  if (p.value > Math.max(1, Math.ceil(history.value.length / pageSize))) p.value = 1;
}

usePageRefresh(refresh);
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
      <Breadcrumbs :items="breadcrumbs" class="mb-4" />
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
