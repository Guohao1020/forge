# Forge Platform

AI 驱动的快速产品孵化平台 —— 让产品经理用自然语言描述需求，AI 生成企业级代码并自动部署。

## 子项目

| 项目 | 定位 | 状态 |
|------|------|------|
| [forge-foundation](./forge-foundation/) | 新一代多版本微服务基座 | Planning |
| [forge-identity](./forge-identity/) | 统一身份认证 + 动态鉴权中心 | Planning |
| [forge-engine](./forge-engine/) | AI 引擎核心（多模型编排 + 代码生成 + Review） | Planning |
| [forge-pipeline](./forge-pipeline/) | DevOps 自动化（云效流水线 + ACK 部署） | Planning |
| [forge-portal](./forge-portal/) | Web 工作台 + 代码可视化 | Planning |
| [forge-bot](./forge-bot/) | IM 机器人（钉钉/飞书） | Planning |
| [forge-beacon](./forge-beacon/) | 实时交互网关（Socket.IO） | Planning |
| [forge-specs](./forge-specs/) | 规范中心（编码规范 + 脚手架模板 + Review 规则） | Planning |

## 文档

- [设计文档](./docs/specs/2026-03-28-forge-platform-design.md) — 完整架构设计与 MVP 路线

## MVP 路线

1. **Phase 1 — 最小闭环**：forge-specs + forge-engine + forge-pipeline + forge-identity(轻) + forge-portal(轻)
2. **Phase 2 — 可用性**：完整 forge-identity + forge-portal
3. **Phase 3 — 扩展性**：forge-foundation + forge-bot + forge-beacon + 多模型路由
4. **Phase 4 — 成熟化**：多版本基座 + 全链路可观测 + 灰度部署
