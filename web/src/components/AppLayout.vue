<script setup lang="ts">
import { computed } from "vue";
import { useRoute } from "vue-router";
import { boltIcon } from "../icon";

defineProps<{ toast?: string }>();
defineEmits<{ toast: [value: string] }>();

const route = useRoute();

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
      <div class="max-w-7xl mx-auto px-6 flex items-center gap-1 -my-[2px]">
        <router-link
          to="/"
          class="text-base mr-4 py-3 font-bold text-body flex items-center gap-1.5"
        >
          <span v-html="boltIcon"></span>
          Runic
        </router-link>
        <router-link
          v-for="t in tabs"
          :key="t.key"
          :to="t.to"
          class="text-sm px-3 py-3 transition border-y-2 border-t-transparent"
          :class="
            activeTab === t.key
              ? 'border-primary text-body font-medium'
              : 'border-transparent text-muted hover:text-body'
          "
        >
          {{ t.label }}
        </router-link>
      </div>
    </nav>
    <main class="max-w-7xl mx-auto p-6">
      <slot />
    </main>
    <div
      v-if="toast"
      class="fixed bottom-6 right-6 z-50 px-4 py-3 bg-primary-solid text-body rounded-lg shadow-lg text-sm font-medium"
    >
      {{ toast }}
    </div>
  </div>
</template>
