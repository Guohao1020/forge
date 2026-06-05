# N4 — daemon/runtime 服务发现 + 配置中心(设计草案)

> 切片 4(草案,**最后做**)。隶属 [Nacos 接入总体方案](README.md)。
> **重叠最深、最 load-bearing、风险最高。原则:只增强 / 双写,绝不替换。**

**Goal:** 用 Nacos 的服务发现 + 配置中心,**标准化**(而非替换)Multica 自带的 `agent_runtime`
注册 + 心跳、`workspace.settings` 动态配置——主要价值在"已用 Nacos 做微服务、想统一基础设施"
或"外部系统要发现 Multica 的 runtime"时。

## 1. 缺口 & 重叠(为什么放最后)

Multica 现有:
- `agent_runtime` 表 + 心跳(`last_seen_at`)= **自带的服务注册 / 发现**,且 claim/dispatch 全靠它,
  **load-bearing**。
- `workspace.settings` JSONB = 动态配置(**无 UI**,daemon register/refresh 时读)。

Nacos 能补:标准服务发现协议、配置热更新、跨系统发现。但**重叠最深**——动它等于动派发命脉,
所以放最后,且只增强。

## 2. Nacos 资源映射(两个独立子能力)

- **服务发现**:把 daemon/runtime 注册成 Nacos service 实例(供外部系统发现 Multica runtime)。
- **配置中心**:把 `workspace.settings` / 部分动态配置放 Nacos config,**热更新**(改配置不重启 daemon)。

## 3. Multica 接入(只增强)

- **服务发现**:**镜像** `agent_runtime` → Nacos service 实例(单向,Multica 仍是真相源 + claim 仍走
  既有心跳)。给外部生态一个发现入口,**不碰** claim/dispatch 路径。
- **配置中心**:`workspace.settings` 可选地由 Nacos config 支撑 + 加一个 settings UI;daemon 订阅
  Nacos config 实现热更新。仍保留 DB 作为 fallback。

## 4. 真相源取向

**坚决只增强 / 双写,不替换**:`agent_runtime` 心跳、claim/dispatch 永远是 Multica 真相源;
Nacos 是"对外发现 + 热更新配置"的叠加层。任何"让 Nacos 当 runtime 真相源"的方案在本切片**否决**
(风险/收益不成正比)。

## 5. 关键决策(深化时定)

- **到底做哪个子能力**:服务发现 / 配置中心 / 两个都做?(很可能只值得做"配置中心 + settings UI",
  服务发现纯锦上添花,除非确有外部消费方。)
- 是否真有外部系统要发现 Multica runtime(没有的话服务发现不做)。
- 配置热更新与 daemon 现有 register/refresh 拉取的关系(订阅 vs 轮询)。

## 6. 依赖 & 非目标

- **依赖**:N0;放在 N1/N2(/N3)之后,等团队对 Nacos 接入有信心再碰 load-bearing 区。
- **非目标(强约束)**:不替换 `agent_runtime` 心跳;不替换 claim/dispatch;不让 Nacos 成为
  runtime 的真相源;不引入"Nacos 挂 = agent 派发停"的硬依赖。
