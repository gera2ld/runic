<script setup lang="ts">
import { ref, computed, onMounted } from "vue";
import AppLayout from "../components/AppLayout.vue";
import ActionCard from "../components/ActionCard.vue";
import { fetchActions } from "../api";
import type { ActionDef } from "../utils";

const actions = ref<ActionDef[]>([]);
const loading = ref(true);

const activeActions = computed(() => actions.value.filter((a) => !a.system && a.active));
const inactiveActions = computed(() => actions.value.filter((a) => !a.system && !a.active));
const systemActions = computed(() => actions.value.filter((a) => a.system));

async function refresh() {
  loading.value = true;
  try {
    const a = await fetchActions();
    actions.value = Array.isArray(a) ? a : [];
  } catch (e) {
    console.error(e);
    actions.value = [];
  }
  loading.value = false;
}

onMounted(() => refresh());
</script>

<template>
  <AppLayout>
    <h1 class="text-2xl font-semibold mb-6">Actions</h1>
    <div v-if="loading" class="text-faint text-sm">Loading...</div>
    <template v-else>
      <h2 class="text-lg font-semibold text-subdued mb-3">
        Active <span class="text-faint font-normal text-sm">({{ activeActions.length }})</span>
      </h2>
      <div
        v-if="activeActions.length === 0"
        class="text-faint text-sm bg-surface rounded-lg border border-line px-4 py-6 text-center mb-8"
      >
        No active actions on this server.
      </div>
      <div v-else class="grid grid-cols-1 sm:grid-cols-2 md:grid-cols-3 gap-3 mb-8">
        <ActionCard v-for="a in activeActions" :key="a.id" :action="a" @triggered="refresh" />
      </div>

      <h2 class="text-lg font-semibold text-subdued mb-3">
        System <span class="text-faint font-normal text-sm">({{ systemActions.length }})</span>
      </h2>
      <div
        v-if="systemActions.length === 0"
        class="text-faint text-sm bg-surface rounded-lg border border-line px-4 py-6 text-center mb-8"
      >
        No system actions.
      </div>
      <div v-else class="grid grid-cols-1 sm:grid-cols-2 md:grid-cols-3 gap-3 mb-8">
        <ActionCard v-for="a in systemActions" :key="a.id" :action="a" @triggered="refresh" />
      </div>

      <h2 class="text-lg font-semibold text-subdued mb-3">
        Inactive <span class="text-faint font-normal text-sm">({{ inactiveActions.length }})</span>
      </h2>
      <div
        v-if="inactiveActions.length === 0"
        class="text-faint text-sm bg-surface rounded-lg border border-line px-4 py-6 text-center"
      >
        No inactive actions.
      </div>
      <div v-else class="grid grid-cols-1 sm:grid-cols-2 md:grid-cols-3 gap-3">
        <ActionCard v-for="a in inactiveActions" :key="a.id" :action="a" @triggered="refresh" />
      </div>
    </template>
  </AppLayout>
</template>
