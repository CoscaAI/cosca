# Python API — Enterprise Grade

> **Version**: 1.0.0 | **Stack**: Python 3.12+, FastAPI, SQLAlchemy, Pydantic v2

## Server

```python
# main.py
from fastapi import FastAPI, Depends, HTTPException, Request
from fastapi.middleware.cors import CORSMiddleware
from contextlib import asynccontextmanager

@asynccontextmanager
async def lifespan(app: FastAPI):
    await init_db()
    yield
    await close_db()

app = FastAPI(lifespan=lifespan)

app.add_middleware(CORSMiddleware, allow_origins=["https://example.com"],
                   allow_methods=["*"], allow_headers=["*"],
                   allow_credentials=True)

@app.get("/health")
async def health(): return {"status": "ok"}

@app.get("/api/v1/users/{user_id}")
async def get_user(user_id: str, db=Depends(get_db), user=Depends(get_current_user)):
    row = await db.fetch_one("SELECT id, name, email FROM users WHERE id = $1", user_id)
    if not row:
        raise HTTPException(404, "user not found")
    return dict(row)
```

## Pydantic Validation

```python
from pydantic import BaseModel, EmailStr, field_validator

class CreateUserInput(BaseModel):
    name: str
    email: EmailStr

    @field_validator("name")
    @classmethod
    def name_not_empty(cls, v: str) -> str:
        v = v.strip()
        if not v or len(v) > 100:
            raise ValueError("name must be 1-100 chars")
        return v
```

## Security

```bash
pip-audit          # dependency vulnerabilities
bandit -r src/     # static analysis
safety check       # known vulnerabilities in installed packages
```

```python
# NEVER: password = "admin123"  in source
# Use: os.environ["DB_PASSWORD"] or python-dotenv
# Rate limiting: slowapi
from slowapi import Limiter
limiter = Limiter(key_func=lambda: "global")
app.state.limiter = limiter
```
