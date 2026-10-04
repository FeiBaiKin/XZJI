import { mergeAttributes, Node } from '@tiptap/core'
import Document from '@tiptap/extension-document'
import { VueNodeViewRenderer } from '@tiptap/vue-3'
import HiddenBlockView from './HiddenBlockView.vue'

declare module '@tiptap/core' {
  interface Commands<ReturnType> {
    hiddenBlock: {
      insertHiddenBlock: (attrs?: { cond?: string, title?: string }) => ReturnType
    }
  }
}

// 隐藏内容块：保存时由后端拆出单独存放，前台按解锁条件显示。
// 单独放在 hidden 分组里，只允许出现在正文顶层，列表、引用里放不进去（后端也会拒绝嵌套）。
export const HiddenBlock = Node.create({
  name: 'hiddenBlock',
  group: 'hidden',
  content: 'block+',
  defining: true,
  isolating: true,

  addAttributes() {
    return {
      cond: {
        default: 'login',
        parseHTML: el => el.getAttribute('data-cond') || 'login',
        renderHTML: attrs => ({ 'data-cond': attrs.cond }),
      },
      title: {
        default: '',
        parseHTML: el => el.getAttribute('data-title') ?? '',
        renderHTML: attrs => ({ 'data-title': attrs.title }),
      },
    }
  },

  parseHTML() {
    return [{ tag: 'div[data-type="hidden-block"]' }]
  },

  renderHTML({ HTMLAttributes }) {
    return ['div', mergeAttributes({ 'data-type': 'hidden-block' }, HTMLAttributes), 0]
  },

  addNodeView() {
    return VueNodeViewRenderer(HiddenBlockView)
  },

  addCommands() {
    return {
      insertHiddenBlock: attrs => ({ commands }) => commands.insertContent({
        type: this.name,
        attrs,
        content: [{ type: 'paragraph' }],
      }),
    }
  },
})

// 正文顶层既可以是普通块，也可以是隐藏块
export const HiddenAwareDocument = Document.extend({ content: '(block|hidden)+' })
