// 页面加载后在浏览器端获取登录状态，不阻塞首屏
export default defineNuxtPlugin(() => {
  void useAuth().ensure()
})
