# Svelte 5 + SvelteKit — Enterprise Grade

> **Version**: 1.0.0 | **Stack**: Svelte 5 (runes), SvelteKit 2, TypeScript

```svelte
<!-- src/routes/login/+page.svelte -->
<script lang="ts">
  let email = $state("")
  let password = $state("")
  let loading = $state(false)
  let error = $state("")

  async function login() {
    if (!email || password.length < 8) return
    loading = true; error = ""
    try {
      const res = await fetch("/api/auth/login", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ email, password }),
      })
      if (!res.ok) throw new Error((await res.json()).message)
      window.location.href = "/dashboard"
    } catch (e) {
      error = e instanceof Error ? e.message : "Login failed"
    } finally {
      loading = false
    }
  }
</script>

<form onsubmit={login} class="flex flex-col gap-4 max-w-md">
  <input bind:value={email} type="email" placeholder="Email" class="border rounded p-2" />
  <input bind:value={password} type="password" placeholder="Password" class="border rounded p-2" />
  {#if error}<p class="text-red-500 text-sm">{error}</p>{/if}
  <button type="submit" disabled={loading} class="bg-blue-600 text-white p-2 rounded">
    {loading ? "Loading..." : "Sign In"}
  </button>
</form>
```

## Security

```bash
pnpm build && pnpm check  # svelte-check for type errors
pnpm audit
```

- `{@html}` only with DOMPurify sanitization
- SvelteKit `+page.server.ts` for server-side auth — never expose tokens to client
- `hooks.server.ts` for global auth guard
