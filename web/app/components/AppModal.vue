<script setup lang="ts">
// 通用弹窗：Esc 或点遮罩关闭
const open = defineModel<boolean>({ required: true })
defineProps<{ title: string }>()

function onKey(e: KeyboardEvent) {
  if (e.key === 'Escape') open.value = false
}
watch(open, (v) => {
  if (v) addEventListener('keydown', onKey)
  else removeEventListener('keydown', onKey)
})
onBeforeUnmount(() => removeEventListener('keydown', onKey))
</script>

<template>
  <Teleport to="body">
    <div v-if="open" class="fixed inset-0 z-40 grid place-items-center bg-[rgba(20,22,28,.45)] px-4" @click.self="open = false">
      <div class="card relative w-full max-w-[440px] px-6 pt-5 pb-6" role="dialog" aria-modal="true" :aria-label="title">
        <button type="button" class="absolute top-3 right-3 grid size-8 place-items-center rounded-full text-muted hover:bg-soft hover:text-title" aria-label="关闭" @click="open = false">
          <Icon name="x" />
        </button>
        <h3 class="text-base font-bold text-title">{{ title }}</h3>
        <slot />
      </div>
    </div>
  </Teleport>
</template>
