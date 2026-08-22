# FRAMEWORK DETECTOR

> **Version**: 1.0.0 | **Status**: active | **Owner**: Cosca Bootstrap

## PURPOSE
Detect web frameworks (frontend, backend, meta-frameworks) by analyzing dependencies and project structure.

## DETECTION RULES

### Frontend Frameworks
| Dependency | Framework | Type |
|-----------|-----------|------|
| next | Next.js | Full-stack Meta |
| nuxt | Nuxt | Full-stack Meta |
| sveltekit, @sveltejs/kit | SvelteKit | Full-stack Meta |
| remix, @remix-run/react | Remix | Full-stack Meta |
| react, react-dom (without next) | React | Frontend |
| vue (without nuxt) | Vue | Frontend |
| @angular/core | Angular | Frontend |
| svelte (without sveltekit) | Svelte | Frontend |
| solid-js | SolidJS | Frontend |
| preact | Preact | Frontend |
| astro | Astro | Static Meta |

### Backend Frameworks
| Dependency | Framework | Language |
|-----------|-----------|----------|
| @nestjs/core | NestJS | TypeScript |
| express | Express | JavaScript |
| fastify | Fastify | JavaScript |
| koa | Koa | JavaScript |
| hono | Hono | JavaScript |
| fastapi | FastAPI | Python |
| django | Django | Python |
| flask | Flask | Python |
| rails | Rails | Ruby |
| sinatra | Sinatra | Ruby |
| spring-boot, spring | Spring | Java |
| gin | Gin | Go |
| echo | Echo | Go |
| fiber | Fiber | Go |
| actix-web | Actix | Rust |
| axum | Axum | Rust |
| rocket | Rocket | Rust |
| phoenix | Phoenix | Elixir |

### UI Libraries (used with frameworks)
| Dependency | Library |
|-----------|--------|
| @shadcn/ui, shadcn-ui | shadcn/ui |
| @radix-ui/* | Radix UI |
| @mui/material, @mui/joy | MUI |
| antd | Ant Design |
| @chakra-ui/react | Chakra UI |
| @mantine/core | Mantine |
| daisyui | DaisyUI |
| tailwindcss | Tailwind CSS |

### State Management
| Dependency | Library |
|-----------|--------|
| zustand | Zustand |
| @reduxjs/toolkit, redux | Redux |
| jotai | Jotai |
| recoil | Recoil |
| mobx | MobX |
| pinia | Pinia (Vue) |
| @tanstack/react-query | TanStack Query |

## DETECTION LOGIC
1. Parse package.json/project file for dependencies
2. Check for meta-framework first (Next.js, Nuxt, SvelteKit)
3. If meta-framework found, it covers both frontend and backend
4. If no meta-framework, check frontend and backend separately
5. Detect UI library and state management as supplementary
6. For non-JS: check project file dependencies

## OUTPUT
```yaml
detection:
  frontend:
    framework: next.js
    ui_library: shadcn/ui
    state_management: zustand
  backend:
    framework: nestjs
    type: rest
  meta:
    framework: next.js
  styling:
    - tailwindcss
    - shadcn/ui
```

## HISTORY
| Version | Date | Author | Changes |
|---------|------|--------|---------|
| 1.0.0 | 2026-07-11 | Cosca Bootstrap | Initial detector |
