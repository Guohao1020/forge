# Nacos 3.x AI Registry — 实测 REST 接口(iris spike,2026-06-06)

最初实测自 `nacos/nacos-server:v3.0.1` standalone(throwaway,临时端口 18848);
随后把 compose 栈固定到 **`v3.2.2`**(与用户官网一键安装版本一致,自带 Web 控制台),
适配集成测试(`TestClientRoundTrip`,register→get→list)对 **v3.2.2 同样通过** ——
本节记录的 AI Registry MCP API 在 3.0.1→3.2.2 间稳定。
适配实现(`client.go`)照本文件填具体 path/payload。

> 端口(compose,见 `docker-compose.selfhost.yml`):宿主 `8848`=REST API/主服务,
> `8849`→镜像内 `8080`=Web 控制台 UI(宿主 8080 被占故映 8849),`9848`=gRPC(SDK)。

## 启动必需 env(Nacos 3.x 强制,即使关 auth)

```
MODE=standalone
NACOS_AUTH_ENABLE=false                 # 关鉴权(dev)
NACOS_AUTH_TOKEN=<base64, 解码 >=32 字节>  # 缺则启动 exit 255
NACOS_AUTH_IDENTITY_KEY=nacos           # 缺则启动 exit 255
NACOS_AUTH_IDENTITY_VALUE=nacos
```

> 结论:计划里 compose 的 4 个 auth env 写法**正确**,已实测验证。

## 鉴权

- admin API(`/nacos/v3/admin/...`)**即使关了 auth 仍要带 identity header**:
  `NACOS_AUTH_IDENTITY_KEY: NACOS_AUTH_IDENTITY_VALUE`,本 spike 即 `nacos: nacos`。
- 不带 header → **403**;带 → 200。

## 通用响应信封

```json
{ "code": 0, "message": "success", "data": <payload> }
```

## 端点

### 列 MCP server
`GET /nacos/v3/admin/ai/mcp/list?namespaceId=<ns>&pageNo=1&pageSize=10`
→ `data`: `{ "totalCount", "pageNumber", "pagesAvailable", "pageItems": [ <McpServer> ] }`(分页)。

### 注册 MCP server
`POST /nacos/v3/admin/ai/mcp`
- 必需参数:`namespaceId`、`mcpName`、**`serverSpecification`(类型 `McpServerBasicInfo`,JSON)**。
- 缺 `serverSpecification` → `400 {"code":10000,"message":"parameter missing",
  "data":"Required parameter 'serverSpecification' type McpServerBasicInfo is not present"}`。
- **待适配实现时定稿** `McpServerBasicInfo` 的确切字段(name/version/protocol/
  frontProtocol/remoteServerConfig 等)+ 可能的 `endpointSpecification` / `toolSpecification`——
  探活的 Nacos 控制台 Network 面板或 Nacos 源码 `McpServerBasicInfo` 是最准来源。

### namespace
`GET /nacos/v3/admin/core/namespace/list` → `data`: `[ { "namespace", "namespaceShowName", ... } ]`。
创建 namespace 端点(给 workspace 建 ns):`POST /nacos/v3/admin/core/namespace`(待定稿参数)。

## 我们的映射(见 N0-N1 spec)

- Multica `namespace = workspace id`(+ `shared`);列/取/注册都带 `namespaceId`。
- `MCPServerShape`(本仓 types.go)是我们**对外的简化 shape**;适配层负责
  `McpServerBasicInfo` ↔ `MCPServerShape` 的双向翻译(env_keys/header_keys 只存 KEY 名,无密)。
- 适配层固定带 identity header。

## McpServerBasicInfo(实测定稿)

注册成功的最小 + 带配置 body(`serverSpecification`,form 参数,JSON 字符串):
```json
{
  "name": "demo",
  "protocol": "stdio",                  // 我们的 transport;remote 用 "mcp-sse"/"http"
  "description": "...",
  "versionDetail": { "version": "1.0.0" },
  "localServerConfig":  { "command": "...", "args": ["..."], "env_keys": ["..."] },  // stdio;原样 round-trip
  "remoteServerConfig": { "url": "...", "header_keys": ["..."] }                     // remote
}
```
读回(get/list pageItems)字段:`id, name, protocol, frontProtocol, description,
versionDetail{version,release_date,is_latest}, version, remoteServerConfig, localServerConfig,
enabled(bool), capabilities, toolSpec, allVersions[], namespaceId`。
**`localServerConfig`/`remoteServerConfig` 是不透明 JSON,原样存取** → 直接放我们的 shape。

## MCPServerShape ↔ McpServerBasicInfo 映射(适配层)

| MCPServerShape | Nacos |
|---|---|
| Name | name |
| Version | versionDetail.version(读回也在顶层 version) |
| Transport | protocol |
| Command/Args/EnvKeys | localServerConfig.{command,args,env_keys}(stdio) |
| URL/HeaderKeys | remoteServerConfig.{url,header_keys}(remote) |
| Lifecycle | enabled(true→"published" / false→"offline") |

## spike 结论

- ✅ Nacos 3.x 可起、AI Registry MCP API `/nacos/v3/admin/ai/mcp/*`、信封 `{code,message,data}`、
  分页 `pageItems`、identity-header 鉴权、**localServerConfig 原样存取** —— **做法 A 可行,schema 已定稿**。
- ⏳ `SetMCPLifecycle`(toggle enabled)的 update 端点未实测;适配先按 PUT 同端点实现,集成测试校验。
- ⏳ prod 开 auth 时需 login 取 accessToken;dev standalone(auth off)只需 identity header,已够第一切片。
