<script setup lang="ts">
import { computed } from "vue";
import { MAX_VISIBLE_PAGES } from "../utils";

const props = defineProps<{
  modelValue: number;
  total: number;
  pageSize: number;
  pageSizeOptions?: number[];
}>();

const emit = defineEmits<{
  "update:modelValue": [value: number];
  "update:pageSize": [value: number];
}>();

const tp = computed(() => Math.max(1, Math.ceil(props.total / props.pageSize)));

const pages = computed(() => {
  const result: number[] = [];
  let start = Math.max(1, props.modelValue - Math.floor(MAX_VISIBLE_PAGES / 2));
  const end = Math.min(tp.value, start + (MAX_VISIBLE_PAGES - 1));
  start = Math.max(1, end - (MAX_VISIBLE_PAGES - 1));
  for (let i = start; i <= end; i++) result.push(i);
  return result;
});
</script>

<template>
  <div
    v-if="tp > 1 || pageSizeOptions"
    class="flex items-center justify-between mb-4 text-sm text-muted"
  >
    <span v-if="tp > 1">
      {{ (modelValue - 1) * pageSize + 1 }}-{{ Math.min(modelValue * pageSize, total) }} of
      {{ total }}
    </span>
    <span v-else> {{ total }} {{ total === 1 ? "result" : "results" }} </span>
    <div class="flex gap-4 items-center">
      <label v-if="pageSizeOptions" class="flex items-center gap-2 text-xs text-muted">
        <span>Per page</span>
        <select
          :value="pageSize"
          @change="emit('update:pageSize', Number(($event.target as HTMLSelectElement).value))"
          class="bg-surface border border-line-muted rounded px-2 py-1 text-xs text-body focus:outline-none focus:ring-1 focus:ring-primary/40"
        >
          <option v-for="size in pageSizeOptions" :key="size" :value="size">{{ size }}</option>
        </select>
      </label>
      <div v-if="tp > 1" class="flex gap-2">
        <button
          @click="emit('update:modelValue', Math.max(1, modelValue - 1))"
          :disabled="modelValue === 1"
          class="px-3 py-1 rounded border border-line-muted disabled:opacity-40 hover:bg-elevated transition"
        >
          Prev
        </button>
        <button
          v-for="n in pages"
          :key="n"
          @click="emit('update:modelValue', n)"
          :class="
            n === modelValue
              ? 'bg-primary-solid text-body'
              : 'border border-line-muted hover:bg-elevated'
          "
          class="px-3 py-1 rounded transition"
        >
          {{ n }}
        </button>
        <button
          @click="emit('update:modelValue', Math.min(tp, modelValue + 1))"
          :disabled="modelValue === tp"
          class="px-3 py-1 rounded border border-line-muted disabled:opacity-40 hover:bg-elevated transition"
        >
          Next
        </button>
      </div>
    </div>
  </div>
</template>
