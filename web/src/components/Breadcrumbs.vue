<script setup lang="ts">
defineProps<{ items: { label: string; to?: string }[] }>();
</script>

<template>
  <nav
    v-if="items && items.length"
    aria-label="Breadcrumb"
    class="flex flex-wrap items-center gap-2 text-xs text-muted"
  >
    <template v-for="(item, index) in items" :key="item.to || item.label">
      <router-link
        v-if="item.to && index < items.length - 1"
        :to="item.to"
        class="hover:text-body transition font-medium"
      >
        {{ item.label }}
      </router-link>
      <span v-else class="font-medium text-body">{{ item.label }}</span>
      <span v-if="index < items.length - 1" class="text-faint">/</span>
    </template>
  </nav>
</template>
