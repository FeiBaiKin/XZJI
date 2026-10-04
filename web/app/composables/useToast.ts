export type Toast = { id: number, text: string, kind: 'ok' | 'error' }

let seq = 0

// 轻量提示：成功或失败各显示 3 秒
export function useToast() {
  const toasts = useState<Toast[]>('toasts', () => [])
  function show(text: string, kind: Toast['kind'] = 'ok') {
    const id = ++seq
    toasts.value = [...toasts.value, { id, text, kind }]
    setTimeout(() => { toasts.value = toasts.value.filter(t => t.id !== id) }, 3000)
  }
  return { toasts, ok: (t: string) => show(t, 'ok'), error: (t: string) => show(t, 'error') }
}
