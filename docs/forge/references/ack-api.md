# ACK / K8s API 能力清单

> **来源**: 阿里云 ACK 官方文档
> **用途**: Forge 平台部署和 AI 孵化产品部署的基础设施操作参考

---

## 1. 操作分层

ACK 操作分为两层 API，各有不同的认证方式和适用场景：

| 层级 | 认证方式 | 适用场景 |
|------|---------|---------|
| K8s 标准 API | KubeConfig + 客户端证书 | 日常部署、服务管理、配置管理 |
| 阿里云 CS API | RAM 用户 AccessKey + SDK 2.0 | 集群级运维、节点扩缩、组件管理 |

---

## 2. K8s 标准 API（日常操作）

通过 KubeConfig 获取认证凭证（客户端证书），使用 K8s 官方 Java SDK（io.kubernetes:client-java）调用。

### 2.1 Forge 平台日常使用的 K8s 资源

| 资源类型 | 用途 |
|---------|------|
| Deployment | 各服务的部署描述（创建、更新、滚动升级、回滚） |
| Service | 服务发现和负载均衡 |
| Ingress | 外部流量入口（APISIX 作为 Ingress Controller） |
| ConfigMap | 非敏感配置存储 |
| Secret | 敏感凭证存储 |
| Namespace | 环境隔离（dev/staging/prod + 临时预览环境） |
| Pod | 查询状态、日志、指标 |
| HorizontalPodAutoscaler | 基于 CPU/内存/自定义指标自动扩缩容 |

### 2.2 forge-pipeline 的典型操作

| 操作 | K8s API |
|------|---------|
| 部署新版本 | 更新 Deployment 的镜像版本（滚动更新） |
| 回滚 | 回退 Deployment 到上一个 revision |
| 扩缩容 | 修改 Deployment replicas 或 HPA 配置 |
| 创建临时环境 | 创建 Namespace + 部署一整套服务 |
| 销毁临时环境 | 删除 Namespace（级联删除所有资源） |
| 查询部署状态 | 读取 Deployment status（availableReplicas、conditions） |
| 查看 Pod 日志 | 读取 Pod logs（构建失败排查） |

---

## 3. 阿里云 CS API（集群级运维）

通过 RAM 用户 AccessKey 认证，使用阿里云 SDK 2.0 调用。

### 3.1 集群管理

| API | 用途 |
|-----|------|
| CreateCluster | 创建 ACK 集群（托管/Serverless/边缘） |
| DeleteCluster | 删除集群及关联资源 |
| ModifyCluster | 修改集群配置 |
| UpgradeCluster | 升级 K8s 版本 |
| DescribeClusterDetail | 查询集群详情 |
| DescribeClustersV1 | 查询集群列表 |
| DescribeClusterResources | 查询集群关联资源（VPC、SLB 等） |

### 3.2 节点池管理

| API | 用途 |
|-----|------|
| CreateClusterNodePool | 创建节点池 |
| ScaleClusterNodePool | 扩容节点池 |
| ModifyClusterNodePool | 修改节点池配置（实例类型、数量等） |
| RemoveNodePoolNodes | 缩容（移除节点） |
| UpgradeClusterNodepool | 升级节点（kubelet、OS、运行时） |
| DescribeClusterNodePools | 查询节点池列表 |
| CreateAutoscalingConfig | 创建弹性伸缩配置 |

### 3.3 KubeConfig 凭证管理

| API | 用途 |
|-----|------|
| DescribeClusterUserKubeconfig | 获取 KubeConfig（forge-pipeline 连接集群用） |
| RevokeK8sClusterKubeConfig | 吊销 KubeConfig（安全轮转） |
| UpdateK8sClusterUserConfigExpire | 更新过期时间 |
| DescribeSubaccountK8sClusterUserConfig | 获取 RAM 用户/角色的 KubeConfig |

### 3.4 组件管理

| API | 用途 |
|-----|------|
| InstallClusterAddons | 安装组件（如 APISIX Ingress Controller） |
| UpgradeClusterAddons | 升级组件版本 |
| ListAddons | 查询可用组件 |
| GetClusterAddonInstance | 查询已安装组件详情 |

### 3.5 安全与巡检

| API | 用途 |
|-----|------|
| ScanClusterVuls | 扫描集群安全漏洞 |
| DescribeClusterVuls | 查询漏洞信息 |
| RunClusterCheck | 发起集群检查（升级前、组件安装前） |
| CreateClusterDiagnosis | 发起集群诊断 |
| DeployPolicyInstance | 部署安全策略规则 |

### 3.6 编排模板

| API | 用途 |
|-----|------|
| CreateTemplate | 创建 K8s YAML 编排模板 |
| DescribeTemplates | 查询模板列表 |
| UpdateTemplate | 更新模板 |

### 3.7 触发器

| API | 用途 |
|-----|------|
| CreateTrigger | 创建触发器（如代码推送后自动重部署 Pod） |
| DescribeTrigger | 查询触发器列表 |

---

## 4. forge-pipeline 对 ACK 的使用方式总结

| 场景 | 使用哪层 API | 说明 |
|------|------------|------|
| 日常部署（滚动更新） | K8s 标准 API | 更新 Deployment 镜像版本 |
| 临时环境创建/销毁 | K8s 标准 API | 创建/删除 Namespace |
| 服务扩缩容 | K8s 标准 API | 修改 replicas 或 HPA |
| 部署状态查询 | K8s 标准 API | 读取 Deployment status |
| Pod 日志查看 | K8s 标准 API | 构建失败排查 |
| 集群节点扩缩 | 阿里云 CS API | 节点池管理 |
| 组件安装/升级 | 阿里云 CS API | 如 APISIX Ingress Controller |
| 安全巡检 | 阿里云 CS API | 定期漏洞扫描 |
| KubeConfig 管理 | 阿里云 CS API | 凭证获取和轮转 |

---

## 5. 认证方式

### K8s API 认证
- 从 ACK 控制台下载 KubeConfig
- 提取客户端证书（client-cert.pem）和客户端密钥（client-key.pem）
- Java SDK：io.kubernetes:client-java，通过 KubeConfig 初始化 ApiClient

### 阿里云 CS API 认证
- 创建 RAM 用户，授予 AliyunCSFullAccess 权限
- 使用 AccessKey ID / AccessKey Secret 初始化 SDK 客户端
- 推荐：Pod 通过 ServiceAccount 绑定 RAM 角色（RRSA），无需硬编码 AccessKey
