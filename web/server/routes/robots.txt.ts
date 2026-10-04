export default defineEventHandler((event) => {
  const { siteUrl } = useRuntimeConfig().public
  setHeader(event, 'content-type', 'text/plain; charset=utf-8')
  return `User-agent: *
Disallow: /user/
Disallow: /login
Disallow: /forgot
Disallow: /pay/

Sitemap: ${siteUrl}/sitemap.xml
`
})
