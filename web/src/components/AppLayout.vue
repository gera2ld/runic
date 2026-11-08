<script setup lang="ts">
import { computed } from "vue";
import { useRoute } from "vue-router";
import { boltIcon } from "../icon";
import { useRefreshButton } from "../refresh";

defineProps<{ toast?: string }>();
defineEmits<{ toast: [value: string] }>();

const route = useRoute();
const { refreshHandler, refreshing, progress, onClick } = useRefreshButton();
// Ring geometry: circumference of the r=9 progress circle.
const RING_C = 2 * Math.PI * 9;

const tabs = [
  { key: "actions", label: "Actions", to: "/actions" },
  { key: "executions", label: "Executions", to: "/executions" },
  { key: "system", label: "System", to: "/system" },
];

const activeTab = computed(() => route.path.split("/")[1]);
</script>

<template>
  <div class="min-h-screen bg-page text-body">
    <nav class="border-b border-line">
      <div class="max-w-7xl mx-auto px-4 sm:px-6 flex items-center gap-0 sm:gap-1 -my-[2px]">
        <router-link
          to="/"
          class="text-base mr-2 sm:mr-4 py-3 font-bold text-body flex items-center gap-1.5 shrink-0"
        >
          <span v-html="boltIcon"></span>
          Runic
        </router-link>
        <router-link
          v-for="t in tabs"
          :key="t.key"
          :to="t.to"
          class="text-sm px-2 sm:px-3 py-3 transition border-y-2 border-t-transparent whitespace-nowrap"
          :class="
            activeTab === t.key
              ? 'border-primary text-body font-medium'
              : 'border-transparent text-muted hover:text-body'
          "
        >
          {{ t.label }}
        </router-link>
        <button
          v-if="refreshHandler"
          @click="onClick"
          :disabled="refreshing"
          title="Refresh (auto every 5s)"
          aria-label="Refresh"
          class="ml-auto shrink-0 relative inline-flex items-center justify-center w-8 h-8 rounded-full text-primary hover:bg-elevated disabled:opacity-40"
        >
          <svg class="w-5 h-5 -rotate-90" viewBox="0 0 24 24" fill="none">
            <circle
              cx="12"
              cy="12"
              r="9"
              stroke="currentColor"
              stroke-width="2"
              class="opacity-20"
            />
            <circle
              cx="12"
              cy="12"
              r="9"
              stroke="currentColor"
              stroke-width="2"
              stroke-linecap="round"
              :stroke-dasharray="RING_C"
              :stroke-dashoffset="RING_C * (1 - progress)"
            />
          </svg>
        </button>
      </div>
    </nav>
    <main class="max-w-7xl mx-auto p-4 sm:p-6">
      <slot />
    </main>
    <div
      v-if="toast"
      class="fixed bottom-4 left-4 right-4 sm:left-auto sm:bottom-6 sm:right-6 z-50 px-4 py-3 bg-primary-solid text-body rounded-lg shadow-lg text-sm font-medium"
    >
      {{ toast }}
    </div>
  </div>
</template>
