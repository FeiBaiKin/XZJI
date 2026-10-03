<template>
  <div class="flex min-h-screen flex-col bg-[radial-gradient(circle_at_8%_0%,#E1EAFF_0,transparent_38%),radial-gradient(circle_at_92%_100%,#EEE5FF_0,transparent_40%)]">
    <header class="bg-white/95 shadow-[0_1px_0_var(--color-line)]">
      <div class="mx-auto flex h-16 max-w-[1200px] items-center px-5 max-md:px-3">
        <AppLogo />
        <NuxtLink to="/" class="ml-auto inline-flex items-center gap-1 text-sm text-body hover:text-theme">
          <Icon name="home" />返回首页
        </NuxtLink>
      </div>
    </header>
    <main class="grid flex-1 place-items-center px-5 py-10 max-md:px-3 max-md:py-5">
      <slot />
    </main>
  </div>
</template>
