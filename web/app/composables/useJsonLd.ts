// 输出 JSON-LD 结构化数据。
// build 里只能放公开字段：隐藏块正文、下载地址这些付费内容永远不能出现在结构化数据里。
export function useJsonLd(build: () => unknown | unknown[]) {
  useHead(() => {
    const blocks = build()
    return {
      script: (Array.isArray(blocks) ? blocks : [blocks]).map(jsonLd => ({
        type: 'application/ld+json',
        innerHTML: JSON.stringify(jsonLd),
      })),
    }
  })
}
