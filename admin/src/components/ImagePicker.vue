<script setup lang="ts">
import { ref } from 'vue'
import { ElMessage } from 'element-plus'
import { uploadImage, type Media } from '../api'

const model = defineModel<Media | null | undefined>({ required: true })
const props = withDefaults(defineProps<{ shape?: 'cover' | 'square', hint?: string }>(), { shape: 'cover', hint: '' })

const input = ref<HTMLInputElement>()
const uploading = ref(false)

async function onFile(e: Event) {
  const file = (e.target as HTMLInputElement).files?.[0]
  ;(e.target as HTMLInputElement).value = ''
  if (!file) return
  uploading.value = true
  try {
    model.value = await uploadImage(file)
  } catch (err) {
    ElMessage.error((err as Error).message)
  } finally {
    uploading.value = false
  }
}
</script>

<template>
  <div class="picker" :class="props.shape">
    <div v-loading="uploading" class="frame" @click="input?.click()">
      <img v-if="model" :src="model.url" alt="">
      <span v-else class="empty">点击上传</span>
    </div>
    <div class="side">
      <p v-if="model" class="meta">{{ model.width }} × {{ model.height }}</p>
      <p v-if="props.hint" class="hint">{{ props.hint }}</p>
      <div class="acts">
        <el-button size="small" @click="input?.click()">{{ model ? '更换' : '上传' }}</el-button>
        <el-button v-if="model" size="small" link type="danger" @click="model = null">移除</el-button>
      </div>
    </div>
    <input ref="input" type="file" accept="image/jpeg,image/png,image/gif,image/webp" hidden @change="onFile">
  </div>
</template>

<style scoped>
.picker { display: flex; gap: 12px; align-items: flex-start; }
.frame { flex: none; width: 160px; aspect-ratio: 16 / 10; border: 1px dashed #DCDFE6; border-radius: 6px; background: #F6F7F9; display: grid; place-items: center; overflow: hidden; cursor: pointer; }
.square .frame { width: 72px; aspect-ratio: 1; border-radius: 16px; }
.frame:hover { border-color: #2F6BFF; }
.frame img { width: 100%; height: 100%; object-fit: cover; }
.empty { font-size: 12px; color: #8A9099; }
.side { min-width: 0; font-size: 12px; line-height: 1.6; color: #8A9099; }
.side p { margin: 0 0 4px; }
.acts { margin-top: 4px; }
</style>
