<script setup lang="ts">
import { ref, watch } from 'vue'
import { EditorContent, useEditor } from '@tiptap/vue-3'
import StarterKit from '@tiptap/starter-kit'
import Image from '@tiptap/extension-image'
import { TableKit } from '@tiptap/extension-table'
import { Placeholder } from '@tiptap/extensions'
import { ElMessage, ElMessageBox } from 'element-plus'
import { uploadImage } from '../api'
import { HiddenAwareDocument, HiddenBlock } from './hiddenBlock'

const model = defineModel<string>({ required: true })

// allowHidden 关掉“隐藏内容”按钮：用户协议、隐私政策这类公开文档用不到
const { allowHidden = true } = defineProps<{ allowHidden?: boolean }>()

const languages = ['plaintext', 'bash', 'nginx', 'php', 'javascript', 'typescript', 'html', 'css', 'json', 'sql', 'python', 'go', 'yaml', 'ini']

const editor = useEditor({
  content: model.value,
  extensions: [
    StarterKit.configure({
      document: false,
      heading: { levels: [2, 3, 4] },
      link: { openOnClick: false, autolink: true, defaultProtocol: 'https' },
    }),
    HiddenAwareDocument,
    HiddenBlock,
    Image,
    TableKit.configure({ table: { resizable: false } }),
    Placeholder.configure({ placeholder: '开始写正文。图片可以直接粘贴或拖进来。' }),
  ],
  editorProps: {
    handlePaste: (_view, event) => insertFiles(event.clipboardData?.files),
    handleDrop: (_view, event) => insertFiles(event.dataTransfer?.files),
  },
  onUpdate: ({ editor }) => {
    model.value = editor.isEmpty ? '' : editor.getHTML()
  },
})

// 外部替换内容（如加载文章）时同步到编辑器
watch(model, (v) => {
  const e = editor.value
  if (!e || v === e.getHTML() || (v === '' && e.isEmpty)) return
  e.commands.setContent(v, { emitUpdate: false })
})

const uploading = ref(0)
const fileInput = ref<HTMLInputElement>()

function insertFiles(files?: FileList | null): boolean {
  const images = [...(files ?? [])].filter(f => f.type.startsWith('image/'))
  if (!images.length) return false
  void (async () => {
    for (const file of images) {
      uploading.value++
      try {
        const m = await uploadImage(file)
        editor.value?.chain().focus().setImage({ src: m.url, alt: '' }).run()
      } catch (err) {
        ElMessage.error((err as Error).message)
      } finally {
        uploading.value--
      }
    }
  })()
  return true
}

function onPickImage(e: Event) {
  const input = e.target as HTMLInputElement
  insertFiles(input.files)
  input.value = ''
}

async function setLink() {
  const e = editor.value
  if (!e) return
  const current = e.getAttributes('link').href as string | undefined
  let url: string
  try {
    ;({ value: url } = await ElMessageBox.prompt('链接地址，留空表示取消链接', '插入链接', {
      inputValue: current ?? 'https://', confirmButtonText: '确定', cancelButtonText: '取消',
    }))
  } catch {
    return
  }
  url = url.trim()
  const chain = e.chain().focus().extendMarkRange('link')
  if (!url || url === 'https://') chain.unsetLink().run()
  else chain.setLink({ href: url }).run()
}

function setLanguage(lang: string) {
  editor.value?.chain().focus().updateAttributes('codeBlock', { language: lang === 'plaintext' ? null : lang }).run()
}

function setBlock(v: string) {
  const c = editor.value?.chain().focus()
  if (!c) return
  if (v === 'p') c.setParagraph().run()
  else c.toggleHeading({ level: Number(v) as 2 | 3 | 4 }).run()
}

function blockValue() {
  const e = editor.value
  for (const level of [2, 3, 4] as const) if (e?.isActive('heading', { level })) return String(level)
  return 'p'
}
</script>

<template>
  <div v-if="editor" class="rich">
    <div class="bar">
      <el-select :model-value="blockValue()" size="small" class="block" @change="setBlock">
        <el-option label="正文" value="p" />
        <el-option label="标题 2" value="2" />
        <el-option label="标题 3" value="3" />
        <el-option label="标题 4" value="4" />
      </el-select>
      <span class="sep" />
      <button type="button" title="加粗" :class="{ on: editor.isActive('bold') }" @click="editor.chain().focus().toggleBold().run()"><b>B</b></button>
      <button type="button" title="斜体" :class="{ on: editor.isActive('italic') }" @click="editor.chain().focus().toggleItalic().run()"><i>I</i></button>
      <button type="button" title="下划线" :class="{ on: editor.isActive('underline') }" @click="editor.chain().focus().toggleUnderline().run()"><u>U</u></button>
      <button type="button" title="删除线" :class="{ on: editor.isActive('strike') }" @click="editor.chain().focus().toggleStrike().run()"><s>S</s></button>
      <button type="button" title="行内代码" :class="{ on: editor.isActive('code') }" @click="editor.chain().focus().toggleCode().run()">&lt;/&gt;</button>
      <button type="button" title="链接" :class="{ on: editor.isActive('link') }" @click="setLink">链接</button>
      <span class="sep" />
      <button type="button" title="无序列表" :class="{ on: editor.isActive('bulletList') }" @click="editor.chain().focus().toggleBulletList().run()">• 列表</button>
      <button type="button" title="有序列表" :class="{ on: editor.isActive('orderedList') }" @click="editor.chain().focus().toggleOrderedList().run()">1. 列表</button>
      <button type="button" title="引用" :class="{ on: editor.isActive('blockquote') }" @click="editor.chain().focus().toggleBlockquote().run()">引用</button>
      <button type="button" title="代码块" :class="{ on: editor.isActive('codeBlock') }" @click="editor.chain().focus().toggleCodeBlock().run()">代码块</button>
      <button type="button" title="分割线" @click="editor.chain().focus().setHorizontalRule().run()">分割线</button>
      <span class="sep" />
      <button type="button" title="插入图片" @click="fileInput?.click()">图片</button>
      <button type="button" title="插入表格" @click="editor.chain().focus().insertTable({ rows: 3, cols: 3, withHeaderRow: true }).run()">表格</button>
      <button v-if="allowHidden" type="button" title="插入只有满足条件才能看到的内容" class="hidden-btn" @click="editor.chain().focus().insertHiddenBlock().run()">隐藏内容</button>
      <span class="sep" />
      <button type="button" title="撤销" :disabled="!editor.can().undo()" @click="editor.chain().focus().undo().run()">撤销</button>
      <button type="button" title="重做" :disabled="!editor.can().redo()" @click="editor.chain().focus().redo().run()">重做</button>
      <span v-if="uploading" class="uploading">正在上传 {{ uploading }} 张图片…</span>
    </div>
    <div v-if="editor.isActive('table')" class="bar sub">
      <span class="label">表格</span>
      <button type="button" @click="editor.chain().focus().addRowAfter().run()">加一行</button>
      <button type="button" @click="editor.chain().focus().addColumnAfter().run()">加一列</button>
      <button type="button" @click="editor.chain().focus().deleteRow().run()">删除行</button>
      <button type="button" @click="editor.chain().focus().deleteColumn().run()">删除列</button>
      <button type="button" class="danger" @click="editor.chain().focus().deleteTable().run()">删除表格</button>
    </div>
    <div v-if="editor.isActive('codeBlock')" class="bar sub">
      <span class="label">代码语言</span>
      <el-select :model-value="editor.getAttributes('codeBlock').language ?? 'plaintext'" size="small" filterable class="lang" @change="setLanguage">
        <el-option v-for="l in languages" :key="l" :label="l" :value="l" />
      </el-select>
    </div>
    <EditorContent :editor="editor" class="doc" />
    <input ref="fileInput" type="file" accept="image/jpeg,image/png,image/gif,image/webp" multiple hidden @change="onPickImage">
  </div>
</template>

<style scoped>
.rich { border: 1px solid #DCDFE6; border-radius: 6px; background: #fff; }
.rich:focus-within { border-color: #2F6BFF; }
.bar { position: sticky; top: 0; z-index: 2; display: flex; flex-wrap: wrap; align-items: center; gap: 2px; padding: 6px 8px; border-bottom: 1px solid #EEF0F3; background: #FAFBFC; border-radius: 6px 6px 0 0; }
.bar.sub { position: static; border-radius: 0; background: #F4F8FF; }
.bar button { height: 28px; min-width: 28px; padding: 0 8px; border: 0; border-radius: 4px; background: none; font-size: 13px; color: #4E5358; cursor: pointer; }
.bar button:hover:not(:disabled) { background: #EBF1FF; color: #2F6BFF; }
.bar button.on { background: #EBF1FF; color: #2F6BFF; font-weight: 600; }
.bar button:disabled { color: #C0C4CC; cursor: not-allowed; }
.bar button.danger { color: #FF4D3D; }
.bar button.hidden-btn { color: #E8590C; }
.sep { width: 1px; height: 16px; margin: 0 4px; background: #E4E7ED; }
.block { width: 90px; }
.lang { width: 140px; }
.label { font-size: 12px; color: #8A9099; margin-right: 6px; }
.uploading { margin-left: auto; font-size: 12px; color: #2F6BFF; }

/* 正文样式尽量接近前台，所见即所得 */
.doc :deep(.tiptap) { min-height: 420px; padding: 16px 20px; outline: none; font-size: 15px; line-height: 1.9; color: #4E5358; }
.doc :deep(.tiptap > * + *) { margin-top: 0.75em; }
.doc :deep(.tiptap p) { margin: 0; }
.doc :deep(.tiptap h2) { position: relative; margin: 1.4em 0 0.5em; padding-left: 14px; font-size: 19px; line-height: 1.4; color: #262A31; }
.doc :deep(.tiptap h2::before) { content: ""; position: absolute; left: 0; top: 3px; bottom: 3px; width: 4px; border-radius: 4px; background: #2F6BFF; }
.doc :deep(.tiptap h3) { margin: 1.2em 0 0.4em; font-size: 17px; color: #262A31; }
.doc :deep(.tiptap h4) { margin: 1em 0 0.3em; font-size: 15px; color: #262A31; }
.doc :deep(.tiptap ul), .doc :deep(.tiptap ol) { padding-left: 1.4em; }
.doc :deep(.tiptap blockquote) { margin: 0; padding: 8px 14px; border-left: 4px solid #D6E2FF; background: #F6F8FC; color: #6B7280; }
.doc :deep(.tiptap code) { padding: 1px 5px; border-radius: 4px; background: #F2F3F5; font-size: 0.9em; color: #D6336C; }
.doc :deep(.tiptap pre) { padding: 12px 14px; border-radius: 6px; background: #22262E; color: #E6E8EB; font-size: 13px; line-height: 1.7; overflow-x: auto; }
.doc :deep(.tiptap pre code) { padding: 0; background: none; color: inherit; }
.doc :deep(.tiptap img) { display: block; max-width: 100%; height: auto; border-radius: 6px; }
.doc :deep(.tiptap img.ProseMirror-selectednode) { outline: 3px solid #2F6BFF; }
.doc :deep(.tiptap a) { color: #2F6BFF; }
.doc :deep(.tiptap hr) { border: 0; border-top: 1px solid #EEF0F3; margin: 1.5em 0; }
.doc :deep(.tiptap table) { width: 100%; border-collapse: collapse; font-size: 14px; }
.doc :deep(.tiptap th), .doc :deep(.tiptap td) { border: 1px solid #E4E7ED; padding: 6px 10px; vertical-align: top; }
.doc :deep(.tiptap th) { background: #F6F7F9; font-weight: 600; text-align: left; }
.doc :deep(.tiptap .selectedCell) { background: #EBF1FF; }
.doc :deep(.tiptap p.is-editor-empty:first-child::before) { content: attr(data-placeholder); float: left; height: 0; color: #B0B6BF; pointer-events: none; }
</style>
