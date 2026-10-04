<script setup lang="ts">
import { computed } from 'vue'
import { NodeViewContent, NodeViewWrapper, nodeViewProps } from '@tiptap/vue-3'
import { condOptions } from '../content'

const props = defineProps(nodeViewProps)
const options = computed(() => condOptions(false))
const cond = computed({
  get: () => props.node.attrs.cond as string,
  set: v => props.updateAttributes({ cond: v }),
})
const title = computed({
  get: () => props.node.attrs.title as string,
  set: v => props.updateAttributes({ title: v }),
})
</script>

<template>
  <NodeViewWrapper class="hb" :class="{ on: selected }">
    <div class="hb-head" contenteditable="false">
      <span class="tag">隐藏内容</span>
      <select v-model="cond" aria-label="解锁条件">
        <option v-for="o in options" :key="o.value" :value="o.value">{{ o.label }}</option>
        <option v-if="!options.some(o => o.value === cond)" :value="cond">{{ cond }}（等级已不存在）</option>
      </select>
      <input v-model="title" maxlength="64" placeholder="未解锁时显示的标题，可以不填" aria-label="隐藏内容标题">
      <button type="button" title="删除这个隐藏块" @click="deleteNode">删除</button>
    </div>
    <NodeViewContent class="hb-body" />
  </NodeViewWrapper>
</template>

<style scoped>
.hb { margin: 12px 0; border: 1.5px dashed #F2B8A8; border-radius: 8px; background: #FFFAF8; }
.hb.on { border-color: #FF8A6B; }
.hb-head { display: flex; flex-wrap: wrap; align-items: center; gap: 8px; padding: 6px 10px; border-bottom: 1px dashed #F2D3C9; font-size: 13px; }
.tag { padding: 1px 8px; border-radius: 4px; background: linear-gradient(135deg, #FF8A6B 10%, #FF3B3B 100%); color: #fff; font-weight: 600; font-size: 12px; }
.hb-head select, .hb-head input { height: 26px; border: 1px solid #E4E7ED; border-radius: 4px; padding: 0 6px; font: inherit; color: #262A31; background: #fff; }
.hb-head input { flex: 1; min-width: 160px; }
.hb-head button { border: 0; background: none; color: #FF4D3D; cursor: pointer; font-size: 13px; }
.hb-body { padding: 4px 14px 8px; }
</style>
