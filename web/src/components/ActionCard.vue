<script setup lang="ts">
import { timeAgo, formatDate, statusClass, formatTimeout } from "../utils";
import { triggerAction } from "../api";
import { actionUrl } from "../urls";
import type { ActionDef } from "../utils";

defineProps<{ action: ActionDef }>();
const emit = defineEmits<{ triggered: [] }>();

function lastRunLabel(a: ActionDef): string {
  return a && a.last_run ? timeAgo(a.last_run) : "-";
}

async function trigger(id: string) {
  const res = await triggerAction(id);
  if (!res.ok) throw new Error(await res.text());
  emit("triggered");
}
</script>

<template>
  <div class="bg-surface border border-line rounded-lg p-4 flex flex-col">
    <div class="flex items-center justify-between mb-2">
      <router-link
        :to="actionUrl(action.id)"
        class="font-semibold text-primary hover:underline truncate"
      >
        {{ action.name || action.id }}
      </router-link>
      <div class="flex gap-1 shrink-0">
        <span
          v-if="action.cron"
          class="text-[10px] px-1.5 py-0.5 bg-accent/10 text-accent-dim rounded font-mono"
          :title="'Cron: ' + action.cron"
          >Cron</span
        >
        <span
          class="text-[10px] px-1.5 py-0.5 bg-subtle text-body rounded font-mono"
          :title="'Concurrency: ' + (action.concurrency === 0 ? 'unlimited' : action.concurrency)"
        >
          {{ action.concurrency === 0 ? "\u221e" : "x" + action.concurrency }}
        </span>
        <span class="text-[10px] px-1.5 py-0.5 bg-subtle text-body rounded font-mono">{{
          formatTimeout(action.timeout)
        }}</span>
      </div>
    </div>
    <div v-if="action.next_run" class="text-[11px] text-muted mb-1">
      Next run: <span class="text-subdued">{{ formatDate(action.next_run) }}</span>
    </div>
    <div class="text-[11px] text-dim mb-3">
      Last run:
      <span class="text-muted" :title="formatDate(action.last_run)">{{
        lastRunLabel(action)
      }}</span>
      <span
        v-if="action.last_run_status"
        :class="statusClass(action.last_run_status)"
        class="ml-2 font-medium"
      >
        {{ action.last_run_status }}
      </span>
    </div>
    <button
      @click="trigger(action.id)"
      class="w-full px-3 py-1.5 bg-primary/20 hover:bg-primary/30 border border-primary/30 rounded text-xs text-primary font-medium transition mt-auto"
    >
      Run
    </button>
  </div>
</template>
