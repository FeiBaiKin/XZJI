<script setup lang="ts">
defineProps<{ icon: string, title: string }>()
</script>

<template>
  <section class="card">
    <h3 class="flex items-center gap-2.5 px-4 pt-3.5 pb-2.5 text-[15px] font-bold text-title">
      <Icon :name="icon" class="size-[17px] text-theme" />{{ title }}
      <span class="ml-auto font-normal"><slot name="extra" /></span>
    </h3>
    <slot />
  </section>
</template>
