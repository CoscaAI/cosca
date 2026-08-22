# Node.js API — Enterprise Grade

> **Version**: 1.0.0 | **Stack**: Node 22, TypeScript, NestJS/Express, Prisma, Zod

## Express + TypeScript

```typescript
import express from "express"
import helmet from "helmet"
import cors from "cors"
import rateLimit from "express-rate-limit"

const app = express()
app.use(helmet())
app.use(cors({ origin: process.env.CORS_ORIGIN, credentials: true }))
app.use(express.json({ limit: "1mb" }))
app.use(rateLimit({ windowMs: 60_000, max: 100 }))

app.get("/health", (_req, res) => res.json({ status: "ok" }))

app.listen(8080)
```

## Zod Validation

```typescript
import { z } from "zod"

const CreateUserSchema = z.object({
  name: z.string().min(1).max(100).trim(),
  email: z.string().email().max(254),
})

app.post("/users", async (req, res) => {
  const input = CreateUserSchema.parse(req.body) // throws on invalid
  // ... create user
})
```

## Security

```bash
pnpm audit           # dependency check
npx eslint .         # lint
```

```typescript
// NEVER: const apiKey = "sk-..."
// Use: process.env.API_KEY with validation
// Helmet sets: CSP, X-Frame-Options, X-Content-Type-Options, HSTS
// Rate limit auth endpoints: 5 req/min per IP
```
