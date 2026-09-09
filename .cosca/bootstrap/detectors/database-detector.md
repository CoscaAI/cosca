# DATABASE DETECTOR

> **Version**: 1.0.0 | **Status**: active | **Owner**: Cosca Bootstrap

## PURPOSE
Detect databases, ORMs, caching layers, and message queues by analyzing dependencies and configuration files.

## DETECTION RULES

### Primary Databases
| Dependency / Config | Database | Certainty |
|---------------------|----------|-----------|
| pg, postgres, postgresql, @prisma/client (postgresql) | PostgreSQL | High |
| mysql, mysql2 | MySQL | High |
| better-sqlite3, sqlite3 | SQLite | High |
| mongodb, mongoose, @prisma/client (mongodb) | MongoDB | High |
| redis, ioredis, @prisma/client (redis) | Redis | High |
| @aws-sdk/client-dynamodb | DynamoDB | Confirmed |
| @google-cloud/firestore | Firestore | Confirmed |
| @supabase/supabase-js | Supabase (PostgreSQL) | Confirmed |
| @neondatabase/serverless | Neon (PostgreSQL) | Confirmed |
| planetscale, @planetscale/database | PlanetScale (MySQL) | Confirmed |
| elasticsearch, @elastic/elasticsearch | Elasticsearch | Confirmed |

### ORMs
| Dependency | ORM |
|-----------|-----|
| prisma, @prisma/client | Prisma |
| typeorm | TypeORM |
| drizzle-orm, drizzle-kit | Drizzle |
| sequelize | Sequelize |
| knex | Knex.js |
| mikro-orm, @mikro-orm/core | MikroORM |
| sqlalchemy | SQLAlchemy (Python) |
| django.db | Django ORM (Python) |
| mongoose | Mongoose (MongoDB) |
| gorm | GORM (Go) |
| diesel | Diesel (Rust) |
| sqlx | SQLx (Rust) |
| ecto | Ecto (Elixir) |

### Migration Tools
| Dependency | Tool |
|-----------|------|
| prisma migrate | Prisma Migrate |
| drizzle-kit | Drizzle Kit |
| typeorm migration | TypeORM CLI |
| sequelize-cli | Sequelize CLI |
| knex migrate | Knex Migrations |
| alembic | Alembic (Python) |
| django migrations | Django Migrations |
| flyway | Flyway (Java) |
| golang-migrate | golang-migrate |
| atlasgo, @ariga/atlas | Atlas |

### Cache & Queue
| Dependency | Type |
|-----------|------|
| redis, ioredis, node-redis | Cache / Queue (Bull/BullMQ) |
| bull, bullmq | Queue (Redis-backed) |
| amqplib, amqp, @nestjs/bullmq | Queue (RabbitMQ) |
| kafkajs, @nestjs/microservices (kafka) | Queue (Kafka) |
| @aws-sdk/client-sqs | Queue (SQS) |
| celery | Queue (Python) |

## OUTPUT
```yaml
detection:
  database:
    primary: postgresql
    host: supabase
  orm:
    name: prisma
    migrations: prisma migrate
  cache:
    - redis
  queue:
    - bullmq
```

## HISTORY
| Version | Date | Author | Changes |
|---------|------|--------|---------|
| 1.0.0 | 2026-07-11 | Cosca Bootstrap | Initial detector |
