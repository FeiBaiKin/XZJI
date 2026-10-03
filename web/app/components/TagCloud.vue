<script setup lang="ts">
import type { TagItem } from '~/utils/content'

defineProps<{ tags: TagItem[] }>()
</script>

<template>
  <div class="flex flex-wrap gap-2 px-4 pb-4">
    <NuxtLink
      v-for="(t, i) in tags"
      :key="t.id"
      :to="tagUrl(t.slug)"
      class="rounded bg-[color-mix(in_srgb,var(--k)_12%,#fff)] px-2.5 py-1 text-[12.5px] text-(--k) hover:bg-(--k) hover:text-white"
      :style="{ '--k': tagColors[i % tagColors.length] }"
    >
      {{ t.name }}
    </NuxtLink>
  </div>
</template>
