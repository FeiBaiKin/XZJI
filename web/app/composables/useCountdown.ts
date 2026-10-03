// 发送验证码后的倒计时
export function useCountdown(seconds = 60) {
  const left = ref(0)
  let timer: ReturnType<typeof setInterval> | undefined

  function start(from = seconds) {
    left.value = from
    clearInterval(timer)
    timer = setInterval(() => {
      if (--left.value <= 0) clearInterval(timer)
    }, 1000)
  }

  onBeforeUnmount(() => clearInterval(timer))
  return { left, start }
}
