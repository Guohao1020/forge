# Forge Platform

AI-driven rapid product incubation platform — product managers describe requirements in natural language, AI generates enterprise-grade code and deploys automatically.

## Sub-projects

| Project | Description | Stack | Status |
|---------|-------------|-------|--------|
| [forge-foundation](./forge-foundation/) | Next-gen multi-version microservice base library | Maven multi-module | Planning |
| [forge-identity](./forge-identity/) | Unified authentication + dynamic authorization center | Java / Spring Boot | Planning |
| [forge-engine](./forge-engine/) | AI engine core (multi-model orchestration + code generation + review) | Java / Spring Boot | Planning |
| [forge-pipeline](./forge-pipeline/) | DevOps automation (CI/CD pipeline + ACK deployment) | Java / Spring Boot | Planning |
| [forge-portal](./forge-portal/) | Web workbench + code visualization | Vue 3 / Vite | Planning |
| [forge-bot](./forge-bot/) | IM bot (DingTalk / Feishu) | Java / Spring Boot | Planning |
| [forge-beacon](./forge-beacon/) | Real-time interaction gateway (Socket.IO) | Node.js | Planning |
| [forge-specs](./forge-specs/) | Standards center (coding conventions + scaffold templates + review rules) | Maven | Planning |

## Quick Start

```bash
# 1. Start infrastructure (MySQL, Redis, Kafka, Nacos, ES, APISIX, etcd)
docker compose up -d

# 2. Build all Java modules
mvn clean compile

# 3. Start the frontend dev server
cd forge-portal && npm install && npm run dev

# 4. Start the real-time gateway
cd forge-beacon && npm install && npm run dev
```

## Documents

- [PRD](docs/PRD.md) — Product requirements
- [Technical Design](docs/technical-design.md) — Architecture and design
- [Milestone Plan](docs/milestone-plan.md) — Delivery roadmap
