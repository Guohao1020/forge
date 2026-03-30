# 脚手架设计范式

> **来源**: solar-foundation 架构模式提炼
> **用途**: Forge 产品加速库复用此设计范式，升级技术栈

---

## 1. 核心设计原则

- **模块化 Starter**：每个能力封装为独立的 starter，按需引入
- **约定优于配置**：开箱即用的合理默认值，需要时可覆盖
- **条件装配**：只在相关依赖存在时才激活组件
- **一键启用**：通过注解一键启用整套框架能力

---

## 2. Starter 两层结构

每个可复用组件遵循两层结构：

```
forge-component-xxx/              ← 父模块（pom 聚合）
├── pom.xml                       ← dependencyManagement
└── forge-component-xxx-starter/  ← starter 模块（type: jar）
    ├── src/
    │   └── main/
    │       ├── java/             ← AutoConfiguration 类
    │       └── resources/
    │           └── META-INF/
    │               └── spring.factories  ← 自动装配入口
    └── pom.xml
```

---

## 3. 自动装配机制

通过 spring.factories（Spring Boot 2.x）或 AutoConfiguration.imports（Spring Boot 3.x）注册自动配置类：

```
# Spring Boot 3.x 方式
META-INF/spring/org.springframework.boot.autoconfigure.AutoConfiguration.imports

每行一个全限定类名
```

---

## 4. 条件激活

自动配置类使用条件注解控制激活时机：

| 注解 | 含义 |
|------|------|
| @ConditionalOnClass | 类路径存在某类时激活 |
| @ConditionalOnProperty | 配置项满足条件时激活 |
| @ConditionalOnMissingBean | 容器中没有同类型 Bean 时激活（允许用户覆盖） |
| @ConditionalOnWebApplication | Web 环境下激活 |

---

## 5. @EnableXxx 注解模式

提供 @EnableXxx 组合注解，一键引入一整套能力：

| 注解 | 包含能力 |
|------|---------|
| @EnableForgeService | 服务发现 + 配置中心 + 日志 + Web + 指标 |
| @EnableForgeGateway | 网关路由 + 服务发现 + 配置中心 |

实现方式：@Import 引入对应的 AutoConfiguration 类组合。

---

## 6. Framework 分层

| 层级 | 用途 |
|------|------|
| forge-component-xxx-starter | 单个能力的最小引入单元 |
| forge-starter-service | 标准微服务（常用组件组合） |
| forge-starter-gateway | API 网关服务 |
| forge-starter-minimal | 最小化服务（仅 common + web） |

---

## 7. 技术栈升级对照

| 维度 | 旧（solar-foundation） | 新（Forge 产品加速库） |
|------|----------------------|---------------------|
| JDK | 8 | 17 |
| Spring Boot | 2.1 | 3.2 |
| Spring Cloud | Greenwich | 2023.x |
| 包路径 | javax.* | jakarta.* |
| 配置中心 | Apollo | Nacos |
| 指标采集 | CAT + InfluxDB | Micrometer + Prometheus |
| 消息队列 | 仅 RocketMQ | Kafka + RocketMQ 双支持 |
| API 文档 | Swagger 2 | SpringDoc OpenAPI 3 |
| 自动装配 | spring.factories | AutoConfiguration.imports |
