<script setup lang="ts">
const { toasts } = useToast()
</script>

<template>
  <div class="pointer-events-none fixed inset-x-0 top-20 z-50 flex flex-col items-center gap-2 px-4" aria-live="polite">
    <p
      v-for="t in toasts"
      :key="t.id"
      class="max-w-[90vw] rounded-lg px-4 py-2.5 text-sm text-white shadow-[0_10px_30px_-10px_rgba(0,0,0,.35)]"
      :class="t.kind === 'ok' ? 'bg-[#262A31]' : 'bg-danger'"
    >
      {{ t.text }}
    </p>
  </div>
</template>
