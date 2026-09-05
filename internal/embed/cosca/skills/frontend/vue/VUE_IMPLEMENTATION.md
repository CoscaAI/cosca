# Vue 3 + Nuxt — Enterprise Grade

> **Version**: 1.0.0 | **Stack**: Vue 3.5, Composition API, Pinia, Nuxt 4

```vue
<!-- pages/login.vue -->
<script setup lang="ts">
const email = ref("")
const password = ref("")
const loading = ref(false)
const error = ref("")

async function login() {
  if (!email.value || password.value.length < 8) return
  loading.value = true
  try {
    await $fetch("/api/auth/login", { method: "POST", body: { email: email.value, password: password.value } })
    navigateTo("/dashboard")
  } catch (e) {
    error.value = e.data?.message || "Login failed"
  } finally {
    loading.value = false
  }
}
</script>

<template>
  <form @submit.prevent="login" class="flex flex-col gap-4 max-w-md">
    <input v-model="email" type="email" placeholder="Email" class="border rounded p-2" />
    <input v-model="password" type="password" placeholder="Password" class="border rounded p-2" />
    <p v-if="error" class="text-red-500 text-sm">{{ error }}</p>
    <button type="submit" :disabled="loading" class="bg-blue-600 text-white p-2 rounded">
      {{ loading ? 'Loading...' : 'Sign In' }}
    </button>
  </form>
</template>
```

## Security

```bash
pnpm build && pnpm test
pnpm audit
```

- `v-html` only with DOMPurify sanitization
- Pinia stores: never store tokens in localStorage
- Nuxt `useCookie` with `httpOnly: true, secure: true, sameSite: 'strict'`
