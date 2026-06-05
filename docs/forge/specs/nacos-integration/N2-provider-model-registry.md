# N2 — LLM Provider / 模型 中心注册表(设计草案)

> 切片 2(草案)。隶属 [Nacos 接入总体方案](README.md)。**架构级**,实施前需按 N0+N1 方式深化。

**Goal:** 把 LLM provider / 端点(像活体执行接的路由器)+ 模型目录集中进 Nacos;agent/runtime
按 `provider_ref` 引用,派发时解析出 provider 环境(Claude `ANTHROPIC_*` / Codex `-c model_providers.*`)
+ 从 Multica 注入密钥。**替代现在 per-agent `custom_env` 散配 base_url/key**。

## 1. 缺口 & 动机

活体执行那一轮暴露的真实痛点:每个 agent 都要在 `custom_env` 里重复配
`ANTHROPIC_BASE_URL` / `ANTHROPIC_AUTH_TOKEN` / `ANTHROPIC_MODEL`(Claude),或在 `custom_args` 里
重复一长串 `-c model_providers.router.*`(Codex)。多个 agent / 多个 provider 时散配、易漂移、
换 key 要逐 agent 改。中心 provider 注册表把 provider 的 **shape** 收一处、版本化,agent 只引用。

## 2. Nacos 资源映射

注册 "LLM provider" 资源(per workspace ns + shared):
- `name`(如 `flatkey-router`)、`version`/`tags`
- `protocol`(`anthropic` / `openai-chat` / `openai-responses` / `bedrock` …)
- `base_url`(占位 `<ROUTER_BASE_URL>`)
- `models`:模型别名/列表(可替代或补 `pkg/agent/models.go` 静态目录)
- `auth`:**密钥的 KEY 名**(如 `ROUTER_API_KEY`),**不存真值**
- 各 provider→runtime-env 的映射规则(Claude 用 `ANTHROPIC_BASE_URL/AUTH_TOKEN/MODEL`;
  Codex 用 `-c model_provider=... -c model_providers.X.base_url=... -c ...env_key=...`)

## 3. Multica 接入

- agent 加 `provider_ref`(引用 Nacos provider)。
- **复用 N1 的 resolver 模式**:派发时 `mcpresolve` 的姊妹 `providerresolve`——按 provider 类型把
  Nacos shape 翻译成该 runtime 需要的 `custom_env`(Claude)或 `custom_args`(Codex)+ 从 Multica
  注入密钥真值。
- 这正好把活体执行手工做的事(给 Inj 注 `ANTHROPIC_*`、给 CodexForge 注 `-c model_providers.router.*`)
  **产品化**成"选一个 provider 即可"。

## 4. 真相源取向

Nacos 当 provider **shape** 真相源(base_url / protocol / 模型别名 / 密钥 KEY 名)、版本化;
**密钥真值留 Multica**(同 N1 的 secret 分离红线)。降级:Nacos 挂 → 缓存上次解析的 provider shape。

## 5. 关键决策(深化时定)

- provider→runtime-env 的映射器,每个 provider 类型一套(Claude / Codex / Bedrock / OpenAI…),
  与 `pkg/agent` 各 backend 的对接点。
- **模型目录**:Nacos 是否接管 `pkg/agent/models.go`(替代/补充)?与 runtime 动态模型发现
  (`/api/runtimes/{id}/models`)的关系。
- `provider_ref` 与既有 `custom_env` / `custom_args` 的优先级(共存;ref 解析结果可被 agent 显式覆盖)。

## 6. 依赖 & 非目标

- **依赖**:N0(基座)。复用 N1 的 resolver + secret 注入 + 缓存降级模式。
- **非目标**:不彻底废除 `custom_env`(共存,留逃生口);不做计费/配额/限流;不做模型路由策略
  (那是路由器自己的事)。
