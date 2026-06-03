<script setup lang="ts">
import { ref, computed, onMounted, watch } from "vue";
import { useRoute } from "vue-router";
import AppLayout from "../components/AppLayout.vue";
import ActionCard from "../components/ActionCard.vue";
import Pagination from "../components/Pagination.vue";
import { fetchActions } from "../api";
import type { ActionDef } from "../utils";

const route = useRoute();
const actions = ref<ActionDef[]>([]);
const loading = ref(true);
const p = ref(1);
const perPage = 10;
const isSystem = computed(() => route.query?.system === "true");

const paged = computed(() => {
  const start = (p.value - 1) * perPage;
  return actions.value.slice(start, start + perPage);
});

async function refresh() {
  loading.value = true;
  try {
    const a = await fetchActions(isSystem.value);
    actions.value = Array.isArray(a) ? a : [];
  } catch (e) {
    console.error(e);
    actions.value = [];
  }
  loading.value = false;
}

onMounted(() => refresh());
watch(
  () => route.query.system,
  () => {
    p.value = 1;
    refresh();
  },
);
</script>

<template>
  <AppLayout>
    <h1 class="text-2xl font-semibold mb-6">{{ isSystem ? "System Actions" : "Actions" }}</h1>
    <div v-if="loading" class="text-faint text-sm">Loading...</div>
    <div
      v-else-if="actions.length === 0"
      class="text-faint text-sm bg-surface rounded-lg border border-line px-4 py-8 text-center"
    >
      No actions found.
    </div>
    <div v-if="!loading && actions.length > 0">
      <Pagination v-model="p" :total="actions.length" :pageSize="perPage" />
      <div class="grid grid-cols-1 sm:grid-cols-2 md:grid-cols-3 gap-3">
        <ActionCard v-for="a in paged" :key="a.id" :action="a" @triggered="refresh" />
      </div>
    </div>
  </AppLayout>
</template>
