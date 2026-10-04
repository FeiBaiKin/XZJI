import { createPublicClient } from '@xzji/api-client/public'

// sitemap 由首页、会员页、分类、标签和内容详情组成，数据从后端的 /seo/sitemap 取。
// 缓存 10 分钟，内容变更时后端会通知清空。
export default defineCachedEventHandler(async (event) => {
  const config = useRuntimeConfig()
  const api = createPublicClient({ baseUrl: config.apiInternalBase })
  const { data, error } = await api.GET('/seo/sitemap')
  if (error || !data) {
    throw createError({ statusCode: 502, statusMessage: '获取 sitemap 数据失败' })
  }
  const site = config.public.siteUrl
  const urls = [
    { loc: `${site}/`, lastmod: '' },
    { loc: `${site}/vip`, lastmod: '' },
    ...data.categories.map(slug => ({ loc: `${site}/category/${encodeURIComponent(slug)}/`, lastmod: '' })),
    ...data.tags.map(slug => ({ loc: `${site}/tag/${encodeURIComponent(slug)}/`, lastmod: '' })),
    ...data.posts.map(p => ({ loc: `${site}/${p.id}.html`, lastmod: new Date(p.updatedAt).toISOString() })),
  ]
  const body = urls.map(u => `  <url><loc>${escapeXml(u.loc)}</loc>${u.lastmod ? `<lastmod>${u.lastmod}</lastmod>` : ''}</url>`)

  setHeader(event, 'content-type', 'application/xml; charset=utf-8')
  return `<?xml version="1.0" encoding="UTF-8"?>
<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">
${body.join('\n')}
</urlset>
`
}, { maxAge: 600 })

function escapeXml(s: string): string {
  return s.replace(/[&<>'"]/g, c => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', "'": '&apos;', '"': '&quot;' })[c]!)
}
