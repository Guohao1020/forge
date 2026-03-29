#!/bin/bash
# Create Nacos namespaces for environment isolation
NACOS_ADDR="http://localhost:8848"

# Namespaces
curl -s -X POST "$NACOS_ADDR/nacos/v1/console/namespaces" -d "customNamespaceId=dev&namespaceName=dev&namespaceDesc=Development"
curl -s -X POST "$NACOS_ADDR/nacos/v1/console/namespaces" -d "customNamespaceId=staging&namespaceName=staging&namespaceDesc=Staging"
curl -s -X POST "$NACOS_ADDR/nacos/v1/console/namespaces" -d "customNamespaceId=prod&namespaceName=prod&namespaceDesc=Production"

# Publish initial configs for dev namespace
# forge-engine
curl -s -X POST "$NACOS_ADDR/nacos/v1/cs/configs" -d "tenant=dev&dataId=forge-engine.yml&group=DEFAULT_GROUP&type=yaml&content=
server:
  port: 8081
spring:
  datasource:
    url: jdbc:mysql://localhost:3306/forge_engine?useSSL=false&allowPublicKeyRetrieval=true&serverTimezone=Asia/Shanghai
    username: root
    password: forge_root_2026
  data:
    redis:
      host: localhost
      port: 6379
      password: forge_redis_2026
  kafka:
    bootstrap-servers: localhost:9094
"

# forge-identity
curl -s -X POST "$NACOS_ADDR/nacos/v1/cs/configs" -d "tenant=dev&dataId=forge-identity.yml&group=DEFAULT_GROUP&type=yaml&content=
server:
  port: 8082
spring:
  datasource:
    url: jdbc:mysql://localhost:3306/forge_identity?useSSL=false&allowPublicKeyRetrieval=true&serverTimezone=Asia/Shanghai
    username: root
    password: forge_root_2026
  data:
    redis:
      host: localhost
      port: 6379
      password: forge_redis_2026
"

# forge-pipeline
curl -s -X POST "$NACOS_ADDR/nacos/v1/cs/configs" -d "tenant=dev&dataId=forge-pipeline.yml&group=DEFAULT_GROUP&type=yaml&content=
server:
  port: 8083
spring:
  datasource:
    url: jdbc:mysql://localhost:3306/forge_pipeline?useSSL=false&allowPublicKeyRetrieval=true&serverTimezone=Asia/Shanghai
    username: root
    password: forge_root_2026
  data:
    redis:
      host: localhost
      port: 6379
      password: forge_redis_2026
"

# forge-specs
curl -s -X POST "$NACOS_ADDR/nacos/v1/cs/configs" -d "tenant=dev&dataId=forge-specs.yml&group=DEFAULT_GROUP&type=yaml&content=
server:
  port: 8084
spring:
  datasource:
    url: jdbc:mysql://localhost:3306/forge_specs?useSSL=false&allowPublicKeyRetrieval=true&serverTimezone=Asia/Shanghai
    username: root
    password: forge_root_2026
"

echo "Nacos namespaces and dev configs initialized."
