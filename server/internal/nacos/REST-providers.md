# Nacos 3.x 配置中心(cs/config)—— LLM provider 实测 REST 接口(prometheus spike,2026-06-06)

实测自 compose 栈固定版本 **`v3.2.2`**(与 N1 MCP spike 同一实例,宿主 `8848`=REST API)。
本文件是 N2(LLM provider registry)的探针产出:测准把 **provider 存进 Nacos 配置中心**
所需的 publish / get / list / history / delete 接口的确切 path / 参数 / 响应信封,
供后续 task 0.4 写 Go 适配(`ProviderQuerier`)时照本填具体调用。

> **为什么用配置中心而非 AI Registry**:实测确认 Nacos AI Registry 只暴露
> `ai/mcp/*`(200),没有 provider / llm / model 资源类型(`ai/provider/list`、
> `ai/llm/list`、`ai/model/list` 全 **404**)。AI Registry 的资源类型是
> MCP / Prompt / Agent / AgentSpec / Skill,**没有原生 "LLM provider" 类型**。
> 因此 provider 存进 **配置中心(cs/config)**:一个 provider = 一条 config,
> `groupName=forge-llm-providers`,`namespaceId=<workspace id 或 "shared">`,
> `content` = 一段 ProviderShape JSON。配置中心原生对每条 config 做历史版本管理
> (拿来当 version/tag);lifecycle 作为 content JSON 里的一个字段。

> 端口同 N1(见 `docker-compose.selfhost.yml`):宿主 `8848`=REST API/主服务,
> `8849`→镜像内 `8080`=Web 控制台 UI,`9848`=gRPC(SDK)。

## 鉴权(与 N1 MCP API 完全一致)

- admin API(`/nacos/v3/admin/...`)**即使关了 auth 仍要带 identity header**:
  本 spike 即 `nacos: nacos`(curl `-H "nacos: nacos"`)。
- 不带 → 403;带 → 200。配置中心和 AI Registry 走同一套 admin 鉴权。

## 通用响应信封

成功:
```json
{ "code": 0, "message": "success", "data": <payload> }
```
config 不存在(get 一个未发布的 dataId,或 get 已删除的 config):
```json
HTTP 404
{ "code": 20004, "message": "resource not found", "data": "Config not exist, please publish Config first." }
```
参数缺失:
```json
HTTP 400
{ "code": 10000, "message": "parameter missing", "data": "Required parameter 'groupName' type String is not present" }
```

## v3 admin path 实测确认

确认 path 在 **`/nacos/v3/admin/cs/config*`** 下(不是旧版 `/nacos/v1/cs/configs`)。
**关键差异:v3 admin 用 `groupName`,不是旧版的 `group`**(用 `group` 会报
`parameter missing: groupName`)。`namespaceId` 名字不变。

## 端点

### publish(发布 / 覆盖一条 config)
`POST /nacos/v3/admin/cs/config`(form / `application/x-www-form-urlencoded`)

form 字段:

| 字段 | 说明 |
|---|---|
| `dataId` | provider 的唯一标识(我们用 provider name,如 `flatkey-router-spike`) |
| `groupName` | 固定 `forge-llm-providers` |
| `namespaceId` | workspace id,或 `shared`;空 → 归一成 `public`(见下) |
| `type` | `json`(让控制台按 JSON 渲染;读回会原样带回 `type` 字段) |
| `content` | ProviderShape JSON 字符串(整段就是这条 config 的内容) |

请求示例(content 用占位符,不放真密钥 / 真 base_url):
```
content={"name":"flatkey-router-spike","version":"1.0.0","protocol":"anthropic","base_url":"<ROUTER_BASE_URL>","auth_key":"ROUTER_API_KEY","lifecycle":"published"}
```
响应:`{"code":0,"message":"success","data":true}`(`data` 是 bool,非对象)。

**publish 是 UPSERT,不是 create-only。** 实测:对同一 `dataId` 二次 publish 仍返回
`{"data":true}`(无 409),且:
- `id`(config 行 id)**不变**;
- `createTime` 不变,`modifyTime` 被刷新;
- `content` 被**整段覆盖**为新值。

→ 这正是 `RegisterProvider` 想要的语义:同名 provider 直接覆盖,不用先查再决定
insert/update。lifecycle 改变(published↔offline)就是改 content 里的字段后再 publish 一次。

### get(读回单条 config —— **是信封,不是裸 content**)
`GET /nacos/v3/admin/cs/config?dataId=<id>&groupName=forge-llm-providers&namespaceId=<ns>`

**响应是 `{code,message,data}` 信封,`data` 是一个元数据对象,真正的 provider JSON
在 `data.content`(是个被转义的字符串,需二次 `json.Unmarshal`)**:
```json
{
  "code": 0, "message": "success",
  "data": {
    "id": "1037378669275783168",      // config 行 id(雪花 id,非 version)
    "namespaceId": "shared",
    "groupName": "forge-llm-providers",
    "dataId": "flatkey-router-spike",
    "md5": "866e31ea23fed4d5e35015748da228be",
    "type": "json",
    "appName": "",
    "createTime": 1780759592339,
    "modifyTime": 1780759629303,
    "desc": null, "configTags": null,
    "encryptedDataKey": "", "createUser": null, "createIp": "172.26.0.1",
    "content": "{\"name\":\"flatkey-router-spike\",\"version\":\"1.0.1\",...}"  // ← ProviderShape,转义字符串
  }
}
```
适配层:先解信封取 `data.content`,再 `Unmarshal(content)` 成 ProviderShape。
`data.id` 是 config 行 id(不是我们要的 version);版本看 history(见下)或直接用
content JSON 里自带的 `version` 字段。

### list(按 group + namespace 列 config —— **pageItems 不带 content**)
`GET /nacos/v3/admin/cs/config/list?namespaceId=<ns>&groupName=forge-llm-providers&pageNo=1&pageSize=10`

响应 `data` 是分页对象,字段名与 N1 MCP list **一致**(`totalCount` / `pageNumber` /
`pagesAvailable` / `pageItems`):
```json
{
  "code": 0, "message": "success",
  "data": {
    "totalCount": 1,
    "pageNumber": 1,
    "pagesAvailable": 1,
    "pageItems": [
      { "id": "...", "namespaceId": "shared", "groupName": "forge-llm-providers",
        "dataId": "flatkey-router-spike", "md5": "...", "type": "json",
        "appName": "", "createTime": 0, "modifyTime": 0, "desc": null, "configTags": null }
    ]
  }
}
```
**注意:`pageItems` 只有元数据,没有 `content` 字段**(list 里 `createTime`/`modifyTime`
甚至回 0)。要拿每条 provider 的实际内容,需对每个 `dataId` 再单发一次 get。
→ `ListProviders` 的实现:list 拿 dataId 集合 → 逐个 get 取 content(或直接复用
分页结果只列名字 + 用 get 拉详情)。

### history(把 config 历史当 "version")
`GET /nacos/v3/admin/cs/history/list?dataId=<id>&groupName=forge-llm-providers&namespaceId=<ns>&pageNo=1&pageSize=10`

每次 publish 产生一条历史。响应分页(同上 `pageItems`),每条带 `opType`:
```json
"pageItems": [
  { "id": "102", ..., "opType": "U", "publishType": "formal", "modifyTime": 1780759629300 },  // 第二次 publish = Update
  { "id": "101", ..., "opType": "I", "publishType": "formal", "modifyTime": 1780759592328 }   // 首次 publish  = Insert
]
```
- `opType`:`I`=insert(首发)、`U`=update(覆盖发布)、`D`=delete。(实测字段值带
  尾部空格 `"U         "`,适配层要 `strings.TrimSpace`。)
- `pageItems[].id`(`102` / `101`)是这条历史的 **nid**,用来取某个历史版本的全文。

取某个历史版本全文(**带 content**):
`GET /nacos/v3/admin/cs/history?dataId=<id>&groupName=forge-llm-providers&namespaceId=<ns>&nid=102`
→ `data` 含完整 `content`(转义字符串)+ `opType` + `extInfo`(如 `{"type":"json"}`)。

→ "version/tag" 方案:用 history 链做版本回溯;**或**更简单 —— 直接用 content JSON
里自带的 `version` 字段(provider 自己声明的语义版本)当对外 version,history 只作审计/回滚。
N2 spec 里 provider version 用 content 内的 `version` 字段即可,history 是兜底审计。

### delete(清理用 —— RegisterProvider/lifecycle 不需要,但记下来)
`DELETE /nacos/v3/admin/cs/config?dataId=<id>&groupName=forge-llm-providers&namespaceId=<ns>`
→ `{"code":0,"message":"success","data":true}`;删后再 get → 404 `code:20004`(见信封节)。
provider 下线走改 content 的 `lifecycle=offline` 后 re-publish(软下线),**不删** config,
保留历史。delete 仅用于 dev 清理 / 测试 teardown。

## namespaceId 空 / public 行为(实测)

- publish 时 `namespaceId=`(空)→ Nacos 归一到默认 namespace **`public`**:
  发布后 get 回来 `data.namespaceId == "public"`,且用 `namespaceId=public` 能取到。
- 即 **空 == `public`**(Nacos 内置默认 namespace)。
- 我们的 `shared` 是一个**显式命名的独立 namespace**(N0 已建),与 `public` 不同;
  适配层永远显式传 `namespaceId`(workspace id 或 `shared`),不要依赖空值。

## 我们的映射(见 N2 spec)

- 一条 config = 一个 provider;`dataId` = provider name,`groupName=forge-llm-providers`,
  `namespaceId` = workspace id(+ `shared` 共享)。
- `content` = ProviderShape JSON(本仓 types.go 待 0.4 定稿),最少含:
  `name` / `version` / `protocol`(anthropic|codex 等)/ `base_url` / `auth_key`(**只存 KEY 名,
  不存密**,与 N1 MCP 的 env_keys/header_keys 同原则)/ `lifecycle`(published|offline)。
- **secret 永不进 Nacos**:`auth_key` 字段存的是环境变量 KEY 名(如 `ROUTER_API_KEY`),
  真值在派发时由本机注入,与 N1 一致。
- 适配层固定带 identity header(`nacos: nacos`)。
- get/list/history 的 `data.content` 都是**转义 JSON 字符串**,需二次 Unmarshal。

## 适配语义速查(给 0.4)

| 适配方法 | Nacos 调用 | 关键点 |
|---|---|---|
| RegisterProvider | `POST cs/config`(form) | UPSERT,同名直接覆盖,返回 `data:true` |
| GetProvider | `GET cs/config` | 信封 → `data.content` 再 Unmarshal;不存在 → 404 `code:20004` |
| ListProviders | `GET cs/config/list` | 分页 `pageItems` 仅元数据,无 content,需逐个 get |
| (version 审计) | `GET cs/history/list` + `GET cs/history?nid=` | `opType` I/U/D(带尾空格);全文在单条 history |
| SetLifecycle(上/下线) | 改 content `lifecycle` 后再 `POST cs/config` | 软下线,不删;走 publish UPSERT |
| DeleteProvider(仅 dev/测试) | `DELETE cs/config` | 生产不用;lifecycle=offline 才是下线 |

## spike 结论

- ✅ v3 admin 配置中心 path 在 `/nacos/v3/admin/cs/config*`,信封 `{code,message,data}`,
  list/history 分页字段名与 MCP API 一致(`pageItems`/`totalCount`),identity-header 鉴权同一套。
- ✅ **publish 是 UPSERT**(同 dataId 覆盖,id 不变 / modifyTime 刷新,无 409)——
  `RegisterProvider` 可直接 "发布即注册",免查询分支。
- ✅ **get 是信封**(provider JSON 在 `data.content`,转义字符串,需二次 Unmarshal),
  **list 的 pageItems 不带 content**(需逐个 get 取详情)。
- ✅ **AI Registry 无 provider 资源类型**(`ai/provider|llm|model/list` 全 404)→ 确认走配置中心。
- ✅ history 可作版本审计(`opType` I/U/D + nid 取全文);对外 version 直接用 content 内的
  `version` 字段更简单。
- ✅ `namespaceId` 空 → 归一 `public`;我们恒显式传 `shared`/workspace id。
- ✅ delete API 存在(`DELETE cs/config`),但 provider 下线用 lifecycle 软下线,delete 仅 dev 清理。
- 🧹 spike config(`flatkey-router-spike` / `nsempty-spike`)已用 delete API 清理,Nacos 无残留。
