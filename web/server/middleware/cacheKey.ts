// 路由缓存的键是完整请求地址（Nitro 的 getKey 用 req.originalUrl 做哈希），utm_*、ref、
// 随机串这些页面用不到的参数会把同一个页面拆成无数个缓存条目。这里把 originalUrl 规范化成
// 只保留页面真正用到的查询参数，再交给路由缓存。
//
// 只改 originalUrl：Nitro 用它取缓存键，而 Nuxt 渲染取的是 event.path（不受影响），
// 所以页面的渲染输入和水合过程与未规范化时完全一致，不会出现水合不一致。
// 推广链接 ?ref= 只在浏览器端读取，仍然有效。

// 各页面真正用到的查询参数，以页面代码为准；不在表里的页面不保留任何查询参数。
const pageParams: { path: RegExp, keep: string[] }[] = [
  // 搜索结果跟着关键词和页码变化
  { path: /^\/search\/?$/, keep: ['q', 'page'] },
  // 分类列表的筛选与排序
  { path: /^\/category\//, keep: ['platform', 'sort', 'access'] },
  // 首页、内容详情 /{id}.html、标签页、会员页、协议页都不读查询参数
  { path: /^\/(?:\d+\.html)?$/, keep: [] },
  { path: /^\/tag\//, keep: [] },
  { path: /^\/vip\/?$/, keep: [] },
  { path: /^\/terms\/?$/, keep: [] },
  { path: /^\/privacy\/?$/, keep: [] },
]

export default defineEventHandler((event) => {
  const req = event.node.req
  const raw = req.originalUrl || req.url || '/'
  const [pathname = '/', search] = raw.split('?')
  if (!search) {
    return
  }
  const rule = pageParams.find(r => r.path.test(pathname))
  if (!rule) {
    return
  }
  const params = new URLSearchParams(search)
  for (const key of [...params.keys()]) {
    if (!rule.keep.includes(key)) {
      params.delete(key)
    }
  }
  const query = params.toString()
  req.originalUrl = query ? `${pathname}?${query}` : pathname
})
