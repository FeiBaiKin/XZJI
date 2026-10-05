// 把路由缓存的存储换成有上限的 LRU 驱动（原因与取值见 utils/cacheLru.ts）。
// Nitro 的路由缓存写在全局 storage 的 cache 挂载点上，这里在启动时把它挂上。
export default defineNitroPlugin(() => {
  try {
    useStorage().mount('cache', createLruDriver())
  } catch (err) {
    // 开发模式下 Nitro 已经用 fs 驱动占了 cache: 挂载点（页面缓存落到 .nuxt/cache），
    // 重复挂载会抛 already mounted；生产构建里没有这个挂载点，挂不上说明真出了问题，
    // 宁可启动失败，也不要悄悄退回到没有上限的内存缓存。
    if (import.meta.dev) {
      return
    }
    throw err
  }
})
