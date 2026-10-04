<template>
  <div class="flex min-h-screen flex-col">
    <AppHeader />
    <main class="mx-auto w-full max-w-[1200px] flex-1 px-5 pt-5 max-md:px-3">
      <slot />
    </main>
    <AppFooter />
    <AppToast />
  </div>
</template>
