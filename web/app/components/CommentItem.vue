<script setup lang="ts">
import type { PublicSchemas } from '@xzji/api-client/public'

type Comment = PublicSchemas['Comment']

defineProps<{ comment: Comment, reply?: boolean }>()
const emit = defineEmits<{ reply: [comment: Comment] }>()
const avatarColors = ['bg-jb-orange', 'bg-jb-purple', 'bg-jb-green', 'bg-jb-blue', 'bg-jb-red']
</script>

<template>
  <div class="flex gap-3" :class="reply ? 'py-2.5' : 'border-b border-line py-3.5 last:border-b-0'">
    <span class="grid shrink-0 place-items-center rounded-full font-bold text-white" :class="[avatarColors[comment.author.id % avatarColors.length], reply ? 'size-7 text-xs' : 'size-9 text-sm']">
      {{ comment.author.nickname.slice(0, 1) }}
    </span>
    <div class="min-w-0 flex-1">
      <b class="text-[13.5px] text-title">
        {{ comment.author.nickname }}
        <em v-if="comment.author.badge" class="ml-1.5 rounded bg-jb-gold px-1.5 py-px align-[1px] text-[11px] font-semibold not-italic text-[#5B3A00]">{{ comment.author.badge }}</em>
        <span v-if="comment.status === 'pending'" class="ml-1.5 rounded bg-[#FFF3E3] px-1.5 py-px text-[11px] font-normal text-[#D97706]">审核中，只有你能看到</span>
      </b>
      <p class="mt-0.5 text-sm break-words whitespace-pre-line"><span v-if="comment.replyTo" class="mr-1 text-theme">@{{ comment.replyTo.nickname }}</span>{{ comment.content }}</p>
      <div class="mt-0.5 flex items-center gap-3 text-xs text-muted">
        <time :datetime="comment.createdAt">{{ relativeTime(comment.createdAt) }}</time>
        <button v-if="comment.status === 'approved'" type="button" class="hover:text-theme" @click="emit('reply', comment)">回复</button>
      </div>
      <slot />
    </div>
  </div>
</template>
