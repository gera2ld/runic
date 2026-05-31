<script setup lang="ts">
import { computed } from "vue";
import { statusClass, formatMs, formatDate, timeAgo } from "../utils";
import { actionUrl, executionUrl } from "../urls";
import type { HistoryEntry } from "../utils";

const props = withDefaults(
  defineProps<{
    entries: HistoryEntry[];
    limit?: number;
    showAction?: boolean;
    showDuration?: boolean;
  }>(),
  { showAction: true, showDuration: false },
);

const visible = computed(() => (props.limit ? props.entries.slice(0, props.limit) : props.entries));
</script>

<template>
  <div class="overflow-x-auto rounded-lg border border-line">
    <table class="w-full text-sm">
      <thead class="bg-surface text-muted uppercase text-xs">
        <tr>
          <th class="px-4 py-3 text-left w-16">ID</th>
          <th v-if="showAction" class="px-4 py-3 text-left">Action</th>
          <th class="px-4 py-3 text-left w-28">Status</th>
          <th v-if="showDuration" class="px-4 py-3 text-left w-24">Duration</th>
          <th class="px-4 py-3 text-left w-48">Started</th>
          <th class="px-4 py-3 text-left w-20">Log</th>
        </tr>
      </thead>
      <tbody class="divide-y divide-rule">
        <tr v-for="e in visible" :key="e.id" class="hover:bg-surface/50 transition">
          <td class="px-4 py-3 font-mono text-faint">{{ e.id }}</td>
          <td v-if="showAction" class="px-4 py-3">
            <router-link
              :to="actionUrl(e.action_id)"
              class="font-mono text-primary hover:underline"
              >{{ e.action_id }}</router-link
            >
          </td>
          <td class="px-4 py-3">
            <span :class="statusClass(e.status)" class="py-1 rounded text-xs font-bold text-body">{{
              e.status
            }}</span>
          </td>
          <td v-if="showDuration" class="px-4 py-3 text-muted">
            {{ formatMs(e.duration_ms) }}
          </td>
          <td class="px-4 py-3 text-dim" :title="formatDate(e.created_at)">
            {{ timeAgo(e.created_at) }}
          </td>
          <td class="px-4 py-3">
            <router-link
              :to="executionUrl(e.action_id, e.id)"
              class="text-accent hover:text-accent-dim text-xs font-medium"
              >View</router-link
            >
          </td>
        </tr>
      </tbody>
    </table>
  </div>
</template>
