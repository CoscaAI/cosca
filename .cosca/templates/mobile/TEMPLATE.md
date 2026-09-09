# MOBILE APP TEMPLATE

> **Version**: 2.0.0 | **Status**: active | **Last Updated**: 2026-07-12

## DOMAIN
Full-stack mobile application with cross-platform mobile client (React Native/Expo or Flutter), REST/GraphQL backend API, shared type contracts, and CI/CD for both app stores. Includes offline-first patterns, push notifications, and mobile-specific security.

## RECOMMENDED STACK

| Layer | Technology | Why |
|-------|-----------|-----|
| Mobile (Primary) | Expo (React Native) + TypeScript | Fast iteration, OTA updates, cross-platform |
| Mobile (Alternative) | Flutter + Dart | Higher performance, custom UI |
| Mobile (Native) | SwiftUI (iOS) / Jetpack Compose (Android) | Platform-specific features |
| Backend API | FastAPI (Python) / NestJS (Node) / Go | REST or GraphQL API |
| Shared Types | Zod (TS) / Pydantic (Python) | Runtime + static validation |
| Database | PostgreSQL + Redis | Reliable + caching/session |
| File Storage | S3 / Cloudflare R2 | Images, uploads |
| Push Notifications | Expo Push / Firebase (FCM + APNs) | Cross-platform push |
| CI/CD Mobile | EAS Build + Fastlane | App store deployment |
| CI/CD Backend | GitHub Actions | Backend deployment |
| Monitoring | Sentry + PostHog | Crash reporting + analytics |

## MODULE STRUCTURE

```
{{PROJECT_NAME}}/
│
├── apps/
│   ├── mobile/                    # Expo Router + TypeScript
│   │   ├── app/                   # File-based routing (Expo Router)
│   │   │   ├── (tabs)/            # Tab navigator screens
│   │   │   │   ├── index.tsx      # Home screen
│   │   │   │   ├── search.tsx     # Search screen
│   │   │   │   ├── profile.tsx    # Profile screen
│   │   │   │   └── _layout.tsx    # Tab layout config
│   │   │   ├── (auth)/            # Auth group (unauthenticated)
│   │   │   │   ├── login.tsx
│   │   │   │   ├── register.tsx
│   │   │   │   ├── forgot-password.tsx
│   │   │   │   └── _layout.tsx    # Auth flow wrapper
│   │   │   ├── (modals)/          # Modal presentations
│   │   │   │   ├── settings.tsx
│   │   │   │   └── _layout.tsx
│   │   │   ├── [id]/              # Dynamic routes
│   │   │   │   └── details.tsx
│   │   │   └── _layout.tsx        # Root layout (auth check)
│   │   ├── src/
│   │   │   ├── components/        # Reusable UI components
│   │   │   │   ├── ui/            # Design system primitives
│   │   │   │   ├── cards/         # Card components
│   │   │   │   ├── forms/         # Form inputs
│   │   │   │   └── feedback/      # Loading, empty, error states
│   │   │   ├── hooks/             # Custom React hooks
│   │   │   ├── stores/            # State management (Zustand)
│   │   │   ├── services/          # API client + business logic
│   │   │   ├── lib/               # Utilities, constants, types
│   │   │   ├── features/          # Feature modules (optional)
│   │   │   │   ├── auth/
│   │   │   │   ├── products/
│   │   │   │   └── orders/
│   │   │   └── providers/         # Context providers
│   │   ├── assets/                # Images, fonts, icons
│   │   ├── app.config.ts          # Expo config
│   │   ├── eas.json               # EAS Build config
│   │   ├── tsconfig.json
│   │   └── package.json
│   │
│   └── api/                       # Backend API (FastAPI / NestJS)
│       ├── src/
│       │   ├── routers/           # API endpoints
│       │   │   ├── auth.py
│       │   │   ├── users.py
│       │   │   └── products.py
│       │   ├── models/            # ORM models
│       │   ├── schemas/           # Pydantic validation schemas
│       │   ├── services/          # Business logic
│       │   ├── middleware/        # Auth, rate limiting, logging
│       │   └── core/
│       │       ├── config.py      # Environment config
│       │       ├── database.py    # DB connection
│       │       └── security.py    # JWT, password hashing
│       ├── tests/
│       ├── requirements.txt
│       └── Dockerfile
│
├── packages/
│   └── shared/                    # Shared types + validation
│       ├── src/
│       │   ├── schemas/           # Zod schemas (shared TS + Python validation)
│       │   ├── types/             # TypeScript type definitions
│       │   └── constants/         # Shared constants
│       ├── tsconfig.json
│       └── package.json
│
├── .github/
│   └── workflows/
│       ├── mobile-ci.yml          # Mobile CI (lint, test, build)
│       ├── api-ci.yml             # Backend CI (lint, test, build)
│       └── deploy.yml             # Deployment workflow
│
├── .cosca/                          # Cosca integration
│   ├── config.yml
│   └── state.yml
│
├── docker-compose.yml             # Local dev: API + DB + Redis
├── .gitignore
└── README.md
```

## KEY FEATURES

### Mobile Client
- File-based routing with Expo Router
- Type-safe navigation with typed routes
- Offline-first with React Query + MMKV persistence
- Push notifications with Expo Push + deep linking
- Biometric auth (FaceID / fingerprint)
- Secure storage with expo-secure-store (Keychain / Keystore)
- Image caching and lazy loading
- Pull-to-refresh and infinite scroll
- Skeleton loading states
- Error boundaries per screen
- Analytics with PostHog
- Crash reporting with Sentry

### Backend API
- FastAPI with async endpoints
- JWT authentication (access + refresh tokens)
- Rate limiting by IP and user
- File upload with S3 presigned URLs
- Push notification dispatch via Expo Push API
- Background tasks with Celery / ARQ
- API versioning (/v1/, /v2/)
- OpenAPI auto-documentation (Swagger UI)
- CORS configured for mobile + web clients
- Health check endpoints (/health, /ready)

### Shared Package
- Zod schemas shared between mobile and API
- Type generation from schemas
- API client with type-safe requests
- Constant values (error codes, status enums)
- Date formatting utilities

### Mobile Security
- Biometric lock for sensitive screens
- Certificate pinning for API calls
- Encrypted local storage (Keychain/Keystore)
- Jailbreak/root detection
- Obfuscated code in production builds
- No secrets in app bundle
- Screen capture prevention for sensitive data

## ARCHITECTURE NOTES

### Offline-First Strategy
```
User Action
    ↓
Optimistic UI Update (immediate)
    ↓
Write to Local DB (MMKV / SQLite)
    ↓
Queue API Request (background)
    ├── Online → Send request → Update remote → Sync
    └── Offline → Queue persists → Retry on connectivity
```

### Push Notification Flow
```
Backend Event (e.g., "new message")
    ↓
API determines target devices
    ↓
Expo Push API / FCM
    ↓
Mobile device receives notification
    ↓
Deep link resolves → navigate to relevant screen
    ↓
If app killed → cold start → deep link → navigate
```

### Auth Flow
```
App Launch
    ↓
Check stored refresh token
    ├── Valid → Refresh access token → Auto-login
    └── Expired/None → Show (auth) group
        ↓
    Login screen
        ↓
    POST /auth/login → JWT access + refresh tokens
        ↓
    Store refresh token in Secure Store
        ↓
    Navigate to (tabs) group
```

### State Management Pattern
```
Screen
    ↓ uses
Custom Hook (useProducts)
    ↓ uses
Zustand Store (productsStore)
    ↓ calls
API Service (productsApi)
    ↓ caches
React Query (useQuery / useMutation)
    ↓ persists
MMKV (offline cache)
```

## BEST PRACTICES

### Do
- Use TypeScript strict mode everywhere
- Share validation schemas between mobile + API
- Implement offline mode for core features
- Handle all states: loading, empty, error, success, edge cases
- Use biometric auth for sensitive operations
- Cache API responses aggressively with TTL
- Optimize images for mobile (WebP, lazy loading)
- Test on real devices (not just simulators)
- Use feature flags for gradual rollouts

### Don't
- Store JWT in AsyncStorage (use Secure Store)
- Hardcode API URLs (use environment config)
- Block UI thread with heavy computation
- Ignore keyboard avoidance on forms
- Assume network is always available
- Skip accessibility labels (VoiceOver/TalkBack)
- Bundle large assets in app binary
- Release without testing on both iOS and Android

## PERMISSIONS MATRIX

| Feature | iOS Permission | Android Permission |
|---------|---------------|-------------------|
| Camera | NSCameraUsageDescription | CAMERA |
| Photo Library | NSPhotoLibraryUsageDescription | READ_MEDIA_IMAGES |
| Push Notifications | Remote Notifications capability | POST_NOTIFICATIONS |
| Biometrics | NSFaceIDUsageDescription | USE_BIOMETRIC |
| Location | NSLocationWhenInUseUsageDescription | ACCESS_FINE_LOCATION |
| Contacts | NSContactsUsageDescription | READ_CONTACTS |

## RELATED
- [Mobile Chief](../../departments/mobile/SKILL.md) — Mobile app development authority
- [Backend Chief](../../departments/backend/SKILL.md) — API implementation
- [UI/UX Chief](../../departments/uiux/SKILL.md) — Mobile design specifications
- [Security Chief](../../departments/security/SKILL.md) — Mobile security review
- [Template Engine](../../engines/templates/SKILL.md) — Scaffolding process
- [RUNTIME_CONTRACT.md](../../RUNTIME_CONTRACT.md) — Runtime interface

## HISTORY

| Version | Date | Author | Changes |
|---------|------|--------|---------|
| 1.0.0 | 2026-07-10 | Cosca Kernel | Initial basic mobile template |
| 2.0.0 | 2026-07-12 | Cosca Kernel | Major expansion: full Expo Router structure, offline-first patterns, push notifications, biometric auth, shared schema packages, architecture diagrams, best practices |
