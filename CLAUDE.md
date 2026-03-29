# Forge Platform — Developer Guide

## Project Structure

- **Java services** (Maven): forge-engine, forge-identity, forge-pipeline, forge-specs, forge-bot
- **Vue 3 frontend** (npm): forge-portal
- **Node.js real-time gateway** (npm): forge-beacon
- **Library** (Maven multi-module): forge-foundation

## Build Commands

```bash
# All Java modules
mvn clean compile

# Single module
cd forge-engine && mvn clean compile

# Frontend
cd forge-portal && npm run build

# Real-time gateway
cd forge-beacon && npm run build
```

## Local Dev Environment

```bash
docker compose up -d    # Start MySQL, Redis, Kafka, Nacos, ES, APISIX, etcd
docker compose down     # Stop all
```

## Ports

| Service | Port |
|---------|------|
| forge-engine | 8081 |
| forge-identity | 8082 |
| forge-pipeline | 8083 |
| forge-specs | 8084 |
| forge-bot | 8085 |
| forge-beacon | 3001 |
| forge-portal (dev) | 5173 |
| MySQL | 3306 |
| Redis | 6379 |
| Kafka (external) | 9094 |
| Nacos | 8848 |
| Elasticsearch | 9200 |
| APISIX (gateway) | 9080 |
| APISIX (admin) | 9180 |

## Coding Standards

See `docs/references/coding-standards.md` for full conventions. Key rules:
- Java 17 + Spring Boot 3.2
- DO/DTO/VO/BO naming convention
- Result<T> for all API responses
- Constructor injection only
- SLF4J with placeholders, no string concat

## Documents

- [PRD](docs/PRD.md) — Product requirements
- [Technical Design](docs/technical-design.md) — Architecture and design
- [Milestone Plan](docs/milestone-plan.md) — Delivery roadmap
