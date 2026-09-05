# Rust API — Enterprise Grade

> **Version**: 1.0.0 | **Stack**: Rust 1.80+, Axum, sqlx, tokio | **Security**: ownership + type safety

## Server Setup

```rust
// src/main.rs
use axum::{Router, middleware, routing::{get, post}, extract::State};
use std::sync::Arc;

#[tokio::main]
async fn main() {
    let db = connect_db().await;
    let state = Arc::new(AppState { db });

    let app = Router::new()
        .route("/health", get(health_check))
        .route("/api/v1/users/{id}", get(get_user))
        .route("/api/v1/users", post(create_user))
        .layer(middleware::from_fn(auth_middleware))
        .layer(tower_http::cors::CorsLayer::permissive()) // tighten in prod
        .layer(tower_http::timeout::TimeoutLayer::new(std::time::Duration::from_secs(30)))
        .with_state(state);

    let listener = tokio::net::TcpListener::bind("0.0.0.0:8080").await.unwrap();
    axum::serve(listener, app).await.unwrap();
}
```

## Handler with Validation

```rust
#[derive(serde::Deserialize)]
struct CreateUserInput {
    name: String,
    email: String,
}

impl CreateUserInput {
    fn validate(&self) -> Result<(), String> {
        if self.name.trim().is_empty() || self.name.len() > 100 {
            return Err("name must be 1-100 chars".into());
        }
        if !self.email.contains('@') || self.email.len() > 254 {
            return Err("invalid email".into());
        }
        Ok(())
    }
}

async fn create_user(
    State(state): State<Arc<AppState>>,
    Json(input): Json<CreateUserInput>,
) -> Result<Json<User>, (StatusCode, String)> {
    input.validate().map_err(|e| (StatusCode::BAD_REQUEST, e))?;

    sqlx::query!("INSERT INTO users (id, name, email) VALUES ($1, $2, $3)",
        uuid::Uuid::new_v4(), input.name, input.email)
        .execute(&state.db)
        .await
        .map_err(|e| (StatusCode::INTERNAL_SERVER_ERROR, e.to_string()))?;

    Ok(Json(User { /* ... */ }))
}
```

## Security

```rust
// NEVER use unsafe { } without audit
// cargo audit — dependency vulnerability scan
// cargo clippy -- -D clippy::all — strict linting
// Secrets via dotenvy: std::env::var("DATABASE_URL") — never hardcoded
// Use secrecy::Secret<String> for sensitive values in memory
```

```bash
cargo build --release
cargo test
cargo clippy -- -D warnings
cargo audit
```
