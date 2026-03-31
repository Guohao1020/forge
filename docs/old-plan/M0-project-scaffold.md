# M0 — 项目骨架 + 基础设施 实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 初始化所有子项目骨架、本地开发基础设施、统一依赖管理，使后续里程碑可以直接在此基础上开发。

**Architecture:** Monorepo 结构，根目录统一 Maven parent pom 管理 Java 子项目版本，forge-portal 为独立 npm 项目。Docker Compose 提供本地开发所需全部中间件。APISIX 作为统一网关入口。

**Tech Stack:** Java 17, Spring Boot 3.2.4, Spring Cloud 2023.0.1, Maven, Vue 3 + TypeScript + Vite, Docker Compose, MySQL 8.0, Redis 7, Kafka 3.7, Nacos 2.3, APISIX 3.9, Elasticsearch 8.13

---

## 文件结构总览

```
forge/
├── pom.xml                          ← 根 parent pom（依赖版本管理）
├── docker-compose.yml               ← 本地开发基础设施
├── docker/
│   ├── apisix/
│   │   ├── config.yaml              ← APISIX 配置
│   │   └── apisix.yaml              ← 路由声明
│   ├── nacos/
│   │   └── custom.env               ← Nacos 环境变量
│   ├── mysql/
│   │   └── init/
│   │       └── 00-init-databases.sql ← 建库脚本
│   └── kafka/
│       └── create-topics.sh         ← Topic 初始化
├── forge-engine/
│   ├── pom.xml
│   └── src/main/java/...            ← 空骨架 + Application 入口
├── forge-identity/
│   ├── pom.xml
│   └── src/main/java/...
├── forge-pipeline/
│   ├── pom.xml
│   └── src/main/java/...
├── forge-specs/
│   ├── pom.xml
│   └── src/main/java/...
├── forge-bot/
│   ├── pom.xml
│   └── src/main/java/...
├── forge-beacon/
│   ├── package.json                 ← Node.js 项目
│   └── src/index.ts
├── forge-portal/
│   ├── package.json                 ← Vue 3 项目
│   ├── vite.config.ts
│   └── src/...
├── forge-foundation/
│   ├── pom.xml                      ← parent 模块
│   └── forge-foundation-starter/
│       └── pom.xml                  ← starter jar（Phase 3 填充）
├── .gitignore                       ← 已有，需补充
├── README.md                        ← 需更新
└── CLAUDE.md                        ← 新建
```

---

### Task 1: 根 Parent POM

**Files:**
- Create: `pom.xml`（根目录）

- [ ] **Step 1: 创建根 pom.xml**

```xml
<?xml version="1.0" encoding="UTF-8"?>
<project xmlns="http://maven.apache.org/POM/4.0.0"
         xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance"
         xsi:schemaLocation="http://maven.apache.org/POM/4.0.0 https://maven.apache.org/xsd/maven-4.0.0.xsd">
    <modelVersion>4.0.0</modelVersion>

    <groupId>com.shulex.forge</groupId>
    <artifactId>forge-platform</artifactId>
    <version>1.0.0-SNAPSHOT</version>
    <packaging>pom</packaging>
    <name>Forge Platform</name>
    <description>AI-driven rapid product incubation platform</description>

    <modules>
        <module>forge-engine</module>
        <module>forge-identity</module>
        <module>forge-pipeline</module>
        <module>forge-specs</module>
        <module>forge-bot</module>
        <module>forge-foundation</module>
    </modules>

    <properties>
        <java.version>17</java.version>
        <maven.compiler.source>17</maven.compiler.source>
        <maven.compiler.target>17</maven.compiler.target>
        <project.build.sourceEncoding>UTF-8</project.build.sourceEncoding>

        <!-- Spring -->
        <spring-boot.version>3.2.4</spring-boot.version>
        <spring-cloud.version>2023.0.1</spring-cloud.version>

        <!-- Database -->
        <mybatis-plus.version>3.5.5</mybatis-plus.version>
        <flyway.version>10.10.0</flyway.version>
        <mysql-connector.version>8.3.0</mysql-connector.version>

        <!-- Cache & MQ -->
        <redisson.version>3.27.2</redisson.version>

        <!-- Nacos -->
        <nacos-config.version>2023.0.1.2</nacos-config.version>
        <nacos-discovery.version>2023.0.1.2</nacos-discovery.version>

        <!-- Tools -->
        <lombok.version>1.18.30</lombok.version>
        <mapstruct.version>1.5.5.Final</mapstruct.version>
        <hutool.version>5.8.26</hutool.version>

        <!-- Test -->
        <testcontainers.version>1.19.7</testcontainers.version>
    </properties>

    <dependencyManagement>
        <dependencies>
            <!-- Spring Boot BOM -->
            <dependency>
                <groupId>org.springframework.boot</groupId>
                <artifactId>spring-boot-dependencies</artifactId>
                <version>${spring-boot.version}</version>
                <type>pom</type>
                <scope>import</scope>
            </dependency>
            <!-- Spring Cloud BOM -->
            <dependency>
                <groupId>org.springframework.cloud</groupId>
                <artifactId>spring-cloud-dependencies</artifactId>
                <version>${spring-cloud.version}</version>
                <type>pom</type>
                <scope>import</scope>
            </dependency>
            <!-- MyBatis-Plus -->
            <dependency>
                <groupId>com.baomidou</groupId>
                <artifactId>mybatis-plus-spring-boot3-starter</artifactId>
                <version>${mybatis-plus.version}</version>
            </dependency>
            <!-- Flyway -->
            <dependency>
                <groupId>org.flywaydb</groupId>
                <artifactId>flyway-core</artifactId>
                <version>${flyway.version}</version>
            </dependency>
            <dependency>
                <groupId>org.flywaydb</groupId>
                <artifactId>flyway-mysql</artifactId>
                <version>${flyway.version}</version>
            </dependency>
            <!-- MySQL -->
            <dependency>
                <groupId>com.mysql</groupId>
                <artifactId>mysql-connector-j</artifactId>
                <version>${mysql-connector.version}</version>
            </dependency>
            <!-- Redisson -->
            <dependency>
                <groupId>org.redisson</groupId>
                <artifactId>redisson-spring-boot-starter</artifactId>
                <version>${redisson.version}</version>
            </dependency>
            <!-- Nacos -->
            <dependency>
                <groupId>com.alibaba.cloud</groupId>
                <artifactId>spring-cloud-starter-alibaba-nacos-config</artifactId>
                <version>${nacos-config.version}</version>
            </dependency>
            <dependency>
                <groupId>com.alibaba.cloud</groupId>
                <artifactId>spring-cloud-starter-alibaba-nacos-discovery</artifactId>
                <version>${nacos-discovery.version}</version>
            </dependency>
            <!-- Lombok -->
            <dependency>
                <groupId>org.projectlombok</groupId>
                <artifactId>lombok</artifactId>
                <version>${lombok.version}</version>
            </dependency>
            <!-- MapStruct -->
            <dependency>
                <groupId>org.mapstruct</groupId>
                <artifactId>mapstruct</artifactId>
                <version>${mapstruct.version}</version>
            </dependency>
            <dependency>
                <groupId>org.mapstruct</groupId>
                <artifactId>mapstruct-processor</artifactId>
                <version>${mapstruct.version}</version>
            </dependency>
            <!-- Hutool -->
            <dependency>
                <groupId>cn.hutool</groupId>
                <artifactId>hutool-all</artifactId>
                <version>${hutool.version}</version>
            </dependency>
            <!-- Testcontainers -->
            <dependency>
                <groupId>org.testcontainers</groupId>
                <artifactId>testcontainers-bom</artifactId>
                <version>${testcontainers.version}</version>
                <type>pom</type>
                <scope>import</scope>
            </dependency>
        </dependencies>
    </dependencyManagement>

    <build>
        <pluginManagement>
            <plugins>
                <plugin>
                    <groupId>org.springframework.boot</groupId>
                    <artifactId>spring-boot-maven-plugin</artifactId>
                    <version>${spring-boot.version}</version>
                    <configuration>
                        <excludes>
                            <exclude>
                                <groupId>org.projectlombok</groupId>
                                <artifactId>lombok</artifactId>
                            </exclude>
                        </excludes>
                    </configuration>
                </plugin>
            </plugins>
        </pluginManagement>
    </build>
</project>
```

- [ ] **Step 2: 验证 pom 格式**

Run: `mvn validate -N` (在根目录，`-N` 不递归到子模块)
Expected: BUILD SUCCESS（子模块还不存在会报 warning 但 validate 不 fail）

- [ ] **Step 3: Commit**

```bash
git add pom.xml
git commit -m "chore(m0): add root parent pom with dependency management"
```

---

### Task 2: Java 子项目骨架（forge-engine）

**Files:**
- Create: `forge-engine/pom.xml`
- Create: `forge-engine/src/main/java/com/shulex/forge/engine/ForgeEngineApplication.java`
- Create: `forge-engine/src/main/resources/application.yml`
- Create: `forge-engine/src/test/java/com/shulex/forge/engine/ForgeEngineApplicationTest.java`

- [ ] **Step 1: 创建 forge-engine/pom.xml**

```xml
<?xml version="1.0" encoding="UTF-8"?>
<project xmlns="http://maven.apache.org/POM/4.0.0"
         xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance"
         xsi:schemaLocation="http://maven.apache.org/POM/4.0.0 https://maven.apache.org/xsd/maven-4.0.0.xsd">
    <modelVersion>4.0.0</modelVersion>

    <parent>
        <groupId>com.shulex.forge</groupId>
        <artifactId>forge-platform</artifactId>
        <version>1.0.0-SNAPSHOT</version>
    </parent>

    <artifactId>forge-engine</artifactId>
    <name>Forge Engine</name>
    <description>AI engine - orchestration and execution</description>

    <dependencies>
        <dependency>
            <groupId>org.springframework.boot</groupId>
            <artifactId>spring-boot-starter-web</artifactId>
        </dependency>
        <dependency>
            <groupId>org.projectlombok</groupId>
            <artifactId>lombok</artifactId>
            <scope>provided</scope>
        </dependency>
        <dependency>
            <groupId>org.springframework.boot</groupId>
            <artifactId>spring-boot-starter-test</artifactId>
            <scope>test</scope>
        </dependency>
    </dependencies>

    <build>
        <plugins>
            <plugin>
                <groupId>org.springframework.boot</groupId>
                <artifactId>spring-boot-maven-plugin</artifactId>
            </plugin>
        </plugins>
    </build>
</project>
```

- [ ] **Step 2: 创建 Application 入口**

`forge-engine/src/main/java/com/shulex/forge/engine/ForgeEngineApplication.java`:
```java
package com.shulex.forge.engine;

import org.springframework.boot.SpringApplication;
import org.springframework.boot.autoconfigure.SpringBootApplication;

@SpringBootApplication
public class ForgeEngineApplication {
    public static void main(String[] args) {
        SpringApplication.run(ForgeEngineApplication.class, args);
    }
}
```

- [ ] **Step 3: 创建 application.yml**

`forge-engine/src/main/resources/application.yml`:
```yaml
server:
  port: 8081

spring:
  application:
    name: forge-engine
```

- [ ] **Step 4: 创建启动测试**

`forge-engine/src/test/java/com/shulex/forge/engine/ForgeEngineApplicationTest.java`:
```java
package com.shulex.forge.engine;

import org.junit.jupiter.api.Test;
import org.springframework.boot.test.context.SpringBootTest;

@SpringBootTest
class ForgeEngineApplicationTest {
    @Test
    void contextLoads() {
    }
}
```

- [ ] **Step 5: 编译验证**

Run: `cd forge-engine && mvn clean compile -q && cd ..`
Expected: BUILD SUCCESS

- [ ] **Step 6: 运行测试**

Run: `cd forge-engine && mvn test -q && cd ..`
Expected: Tests run: 1, Failures: 0

- [ ] **Step 7: Commit**

```bash
git add forge-engine/
git commit -m "chore(m0): scaffold forge-engine with Spring Boot 3.2"
```

---

### Task 3: Java 子项目骨架（forge-identity / forge-pipeline / forge-specs / forge-bot）

对以下 4 个子项目重复 Task 2 的结构，差异仅为 artifactId / name / port / package：

| 子项目 | artifactId | package | port | description |
|--------|-----------|---------|------|-------------|
| forge-identity | forge-identity | com.shulex.forge.identity | 8082 | Auth center |
| forge-pipeline | forge-pipeline | com.shulex.forge.pipeline | 8083 | DevOps automation |
| forge-specs | forge-specs | com.shulex.forge.specs | 8084 | Standards center |
| forge-bot | forge-bot | com.shulex.forge.bot | 8085 | IM bot |

**Files:**（每个子项目）
- Create: `{project}/pom.xml`
- Create: `{project}/src/main/java/com/shulex/forge/{name}/{ClassName}Application.java`
- Create: `{project}/src/main/resources/application.yml`
- Create: `{project}/src/test/java/com/shulex/forge/{name}/{ClassName}ApplicationTest.java`

- [ ] **Step 1: 创建 4 个子项目的 pom.xml + Application + yml + Test**（同 Task 2 模式）
- [ ] **Step 2: 根目录 `mvn clean compile -q`**

Expected: BUILD SUCCESS（6 个 Java 模块全部编译通过）

- [ ] **Step 3: 根目录 `mvn test -q`**

Expected: Tests run: 6, Failures: 0

- [ ] **Step 4: Commit**

```bash
git add forge-identity/ forge-pipeline/ forge-specs/ forge-bot/
git commit -m "chore(m0): scaffold forge-identity, forge-pipeline, forge-specs, forge-bot"
```

---

### Task 4: forge-foundation 骨架（Maven parent + starter 空壳）

**Files:**
- Create: `forge-foundation/pom.xml`（parent 模块）
- Create: `forge-foundation/forge-foundation-starter/pom.xml`（starter jar，暂无代码）

- [ ] **Step 1: 创建 forge-foundation/pom.xml**

```xml
<?xml version="1.0" encoding="UTF-8"?>
<project xmlns="http://maven.apache.org/POM/4.0.0"
         xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance"
         xsi:schemaLocation="http://maven.apache.org/POM/4.0.0 https://maven.apache.org/xsd/maven-4.0.0.xsd">
    <modelVersion>4.0.0</modelVersion>

    <parent>
        <groupId>com.shulex.forge</groupId>
        <artifactId>forge-platform</artifactId>
        <version>1.0.0-SNAPSHOT</version>
    </parent>

    <artifactId>forge-foundation</artifactId>
    <packaging>pom</packaging>
    <name>Forge Foundation</name>
    <description>Reusable component library for AI-incubated products</description>

    <modules>
        <module>forge-foundation-starter</module>
    </modules>
</project>
```

- [ ] **Step 2: 创建 forge-foundation/forge-foundation-starter/pom.xml**

```xml
<?xml version="1.0" encoding="UTF-8"?>
<project xmlns="http://maven.apache.org/POM/4.0.0"
         xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance"
         xsi:schemaLocation="http://maven.apache.org/POM/4.0.0 https://maven.apache.org/xsd/maven-4.0.0.xsd">
    <modelVersion>4.0.0</modelVersion>

    <parent>
        <groupId>com.shulex.forge</groupId>
        <artifactId>forge-foundation</artifactId>
        <version>1.0.0-SNAPSHOT</version>
    </parent>

    <artifactId>forge-foundation-starter</artifactId>
    <name>Forge Foundation Starter</name>
    <description>Auto-configuration starter for forge-foundation</description>

    <dependencies>
        <dependency>
            <groupId>org.springframework.boot</groupId>
            <artifactId>spring-boot-starter</artifactId>
        </dependency>
    </dependencies>
</project>
```

- [ ] **Step 3: 编译验证**

Run: `mvn clean compile -q`
Expected: BUILD SUCCESS（全部 8 个模块）

- [ ] **Step 4: Commit**

```bash
git add forge-foundation/
git commit -m "chore(m0): scaffold forge-foundation parent + starter shell"
```

---

### Task 5: forge-portal 骨架（Vue 3）

**Files:**
- Create: `forge-portal/package.json`
- Create: `forge-portal/vite.config.ts`
- Create: `forge-portal/tsconfig.json`
- Create: `forge-portal/index.html`
- Create: `forge-portal/src/main.ts`
- Create: `forge-portal/src/App.vue`

- [ ] **Step 1: 使用 Vite 初始化**

```bash
cd forge-portal
npm create vite@latest . -- --template vue-ts
```

如果目录非空导致失败，手动创建 package.json 和配置文件。

- [ ] **Step 2: 安装依赖**

```bash
cd forge-portal
npm install
npm install ant-design-vue@4 pinia vue-router@4 @vueuse/core axios
npm install -D @types/node
```

- [ ] **Step 3: 验证构建**

Run: `cd forge-portal && npm run build`
Expected: 构建成功，生成 dist/ 目录

- [ ] **Step 4: Commit**

```bash
git add forge-portal/
git commit -m "chore(m0): scaffold forge-portal with Vue 3 + TypeScript + Vite"
```

---

### Task 6: forge-beacon 骨架（Node.js Socket.IO）

**Files:**
- Create: `forge-beacon/package.json`
- Create: `forge-beacon/tsconfig.json`
- Create: `forge-beacon/src/index.ts`

- [ ] **Step 1: 初始化 package.json**

```bash
cd forge-beacon
npm init -y
npm install socket.io express
npm install -D typescript @types/node @types/express ts-node nodemon
npx tsc --init --target ES2022 --module NodeNext --moduleResolution NodeNext --outDir dist --rootDir src --strict true
```

- [ ] **Step 2: 创建入口 src/index.ts**

```typescript
import express from 'express';
import { createServer } from 'http';
import { Server } from 'socket.io';

const app = express();
const server = createServer(app);
const io = new Server(server, {
  cors: { origin: '*' }
});

const PORT = process.env.PORT || 3001;

app.get('/health', (_req, res) => {
  res.json({ status: 'ok', service: 'forge-beacon' });
});

io.on('connection', (socket) => {
  console.log(`client connected: ${socket.id}`);
  socket.on('disconnect', () => {
    console.log(`client disconnected: ${socket.id}`);
  });
});

server.listen(PORT, () => {
  console.log(`forge-beacon listening on port ${PORT}`);
});
```

- [ ] **Step 3: 在 package.json 中添加 scripts**

```json
{
  "scripts": {
    "dev": "npx ts-node src/index.ts",
    "build": "tsc",
    "start": "node dist/index.js"
  }
}
```

- [ ] **Step 4: 验证编译**

Run: `cd forge-beacon && npm run build`
Expected: 编译成功，生成 dist/index.js

- [ ] **Step 5: Commit**

```bash
git add forge-beacon/
git commit -m "chore(m0): scaffold forge-beacon with Socket.IO + TypeScript"
```

---

### Task 7: Docker Compose 本地开发基础设施

**Files:**
- Create: `docker-compose.yml`
- Create: `docker/mysql/init/00-init-databases.sql`
- Create: `docker/kafka/create-topics.sh`
- Create: `docker/nacos/custom.env`

- [ ] **Step 1: 创建 docker-compose.yml**

```yaml
version: '3.8'

services:
  mysql:
    image: mysql:8.0
    container_name: forge-mysql
    environment:
      MYSQL_ROOT_PASSWORD: forge_root_2026
      MYSQL_CHARACTER_SET_SERVER: utf8mb4
      MYSQL_COLLATION_SERVER: utf8mb4_unicode_ci
    ports:
      - "3306:3306"
    volumes:
      - forge-mysql-data:/var/lib/mysql
      - ./docker/mysql/init:/docker-entrypoint-initdb.d
    command: --default-authentication-plugin=mysql_native_password

  redis:
    image: redis:7-alpine
    container_name: forge-redis
    ports:
      - "6379:6379"
    command: redis-server --requirepass forge_redis_2026

  nacos:
    image: nacos/nacos-server:v2.3.1
    container_name: forge-nacos
    environment:
      MODE: standalone
      SPRING_DATASOURCE_PLATFORM: mysql
      MYSQL_SERVICE_HOST: mysql
      MYSQL_SERVICE_PORT: 3306
      MYSQL_SERVICE_DB_NAME: forge_nacos
      MYSQL_SERVICE_USER: root
      MYSQL_SERVICE_PASSWORD: forge_root_2026
    ports:
      - "8848:8848"
      - "9848:9848"
    depends_on:
      - mysql

  kafka:
    image: bitnami/kafka:3.7
    container_name: forge-kafka
    environment:
      KAFKA_CFG_NODE_ID: 0
      KAFKA_CFG_PROCESS_ROLES: controller,broker
      KAFKA_CFG_CONTROLLER_QUORUM_VOTERS: 0@kafka:9093
      KAFKA_CFG_LISTENERS: PLAINTEXT://:9092,CONTROLLER://:9093,EXTERNAL://:9094
      KAFKA_CFG_ADVERTISED_LISTENERS: PLAINTEXT://kafka:9092,EXTERNAL://localhost:9094
      KAFKA_CFG_LISTENER_SECURITY_PROTOCOL_MAP: CONTROLLER:PLAINTEXT,PLAINTEXT:PLAINTEXT,EXTERNAL:PLAINTEXT
      KAFKA_CFG_CONTROLLER_LISTENER_NAMES: CONTROLLER
      KAFKA_CFG_AUTO_CREATE_TOPICS_ENABLE: "false"
    ports:
      - "9094:9094"

  elasticsearch:
    image: docker.elastic.co/elasticsearch/elasticsearch:8.13.0
    container_name: forge-es
    environment:
      discovery.type: single-node
      xpack.security.enabled: "false"
      ES_JAVA_OPTS: "-Xms512m -Xmx512m"
    ports:
      - "9200:9200"
    volumes:
      - forge-es-data:/usr/share/elasticsearch/data

  apisix:
    image: apache/apisix:3.9.0-debian
    container_name: forge-apisix
    ports:
      - "9080:9080"
      - "9443:9443"
      - "9180:9180"
    volumes:
      - ./docker/apisix/config.yaml:/usr/local/apisix/conf/config.yaml
      - ./docker/apisix/apisix.yaml:/usr/local/apisix/conf/apisix.yaml
    depends_on:
      - etcd

  etcd:
    image: bitnami/etcd:3.5
    container_name: forge-etcd
    environment:
      ALLOW_NONE_AUTHENTICATION: "yes"
    ports:
      - "2379:2379"

volumes:
  forge-mysql-data:
  forge-es-data:
```

- [ ] **Step 2: 创建 MySQL 初始化脚本**

`docker/mysql/init/00-init-databases.sql`:
```sql
CREATE DATABASE IF NOT EXISTS forge_nacos DEFAULT CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;
CREATE DATABASE IF NOT EXISTS forge_engine DEFAULT CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;
CREATE DATABASE IF NOT EXISTS forge_identity DEFAULT CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;
CREATE DATABASE IF NOT EXISTS forge_pipeline DEFAULT CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;
CREATE DATABASE IF NOT EXISTS forge_specs DEFAULT CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;
```

- [ ] **Step 3: 创建 Kafka topic 初始化脚本**

`docker/kafka/create-topics.sh`:
```bash
#!/bin/bash
# Run after kafka is ready:
# docker exec forge-kafka bash /opt/bitnami/kafka/bin/kafka-topics.sh ...

KAFKA_BIN="/opt/bitnami/kafka/bin"
BOOTSTRAP="localhost:9092"

$KAFKA_BIN/kafka-topics.sh --create --bootstrap-server $BOOTSTRAP --topic forge.task.dispatch --partitions 6 --replication-factor 1 --if-not-exists
$KAFKA_BIN/kafka-topics.sh --create --bootstrap-server $BOOTSTRAP --topic forge.task.result --partitions 6 --replication-factor 1 --if-not-exists
$KAFKA_BIN/kafka-topics.sh --create --bootstrap-server $BOOTSTRAP --topic forge.task.event --partitions 3 --replication-factor 1 --if-not-exists

echo "Kafka topics created."
```

- [ ] **Step 4: 创建 APISIX 配置**

`docker/apisix/config.yaml`:
```yaml
apisix:
  node_listen: 9080
  enable_admin: true
  admin_key:
    - name: admin
      key: forge-apisix-admin-2026
      role: admin

deployment:
  role: traditional
  role_traditional:
    config_provider: etcd
  etcd:
    host:
      - "http://etcd:2379"
```

`docker/apisix/apisix.yaml`:
```yaml
# Declarative routes — will be expanded as services come online
routes: []
upstreams: []
#END
```

- [ ] **Step 5: 验证 Docker Compose 启动**

Run: `docker compose up -d`
Expected: 所有 7 个容器启动成功

Run: `docker compose ps`
Expected: 全部 STATUS 为 running/healthy

- [ ] **Step 6: 验证各服务可达**

```bash
# MySQL
docker exec forge-mysql mysql -uroot -pforge_root_2026 -e "SHOW DATABASES;"
# Redis
docker exec forge-redis redis-cli -a forge_redis_2026 PING
# Nacos (等待 30s 启动)
curl -s http://localhost:8848/nacos/v1/console/health
# Elasticsearch
curl -s http://localhost:9200/_cluster/health
# APISIX
curl -s http://localhost:9080/apisix/status
```

- [ ] **Step 7: 创建 Kafka topics**

```bash
docker cp docker/kafka/create-topics.sh forge-kafka:/tmp/
docker exec forge-kafka bash /tmp/create-topics.sh
```

- [ ] **Step 8: Commit**

```bash
git add docker-compose.yml docker/
git commit -m "chore(m0): add Docker Compose local dev infrastructure"
```

---

### Task 8: .gitignore 补充 + README 更新 + CLAUDE.md

**Files:**
- Modify: `.gitignore`
- Modify: `README.md`
- Create: `CLAUDE.md`

- [ ] **Step 1: 补充 .gitignore**

追加以下内容到现有 `.gitignore`：
```
# Docker
docker/mysql/data/
docker/es/data/

# Maven
.mvn/
mvnw
mvnw.cmd

# Build
build/
out/

# Env
.env
*.env.local
```

- [ ] **Step 2: 更新 README.md**

更新 README 使其与当前文档对齐（移除 MVP 表述、更新文档链接、更新 Phase 描述）。

- [ ] **Step 3: 创建 CLAUDE.md**

```markdown
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
```

- [ ] **Step 4: Commit**

```bash
git add .gitignore README.md CLAUDE.md
git commit -m "chore(m0): update gitignore, README, add CLAUDE.md"
```

---

### Task 9: Nacos 命名空间和分组规划

**Files:**
- Create: `docker/nacos/init-namespaces.sh`

- [ ] **Step 1: 创建 Nacos 命名空间初始化脚本**

`docker/nacos/init-namespaces.sh`:
```bash
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
```

- [ ] **Step 2: 执行初始化**

Run: `bash docker/nacos/init-namespaces.sh`
Expected: 3 个 namespace 创建成功 + 4 个 dev 配置发布成功

- [ ] **Step 3: 验证**

打开 http://localhost:8848/nacos → 配置管理 → dev 命名空间 → 看到 4 个配置文件

- [ ] **Step 4: Commit**

```bash
git add docker/nacos/
git commit -m "chore(m0): add Nacos namespace and dev config initialization"
```

---

### Task 10: APISIX 基础路由配置

**Files:**
- Modify: `docker/apisix/apisix.yaml`

- [ ] **Step 1: 添加各服务路由**

更新 `docker/apisix/apisix.yaml`：
```yaml
routes:
  - uri: /api/engine/*
    upstream:
      type: roundrobin
      nodes:
        "host.docker.internal:8081": 1
    plugins:
      proxy-rewrite:
        regex_uri: ["^/api/engine/(.*)", "/$1"]

  - uri: /api/identity/*
    upstream:
      type: roundrobin
      nodes:
        "host.docker.internal:8082": 1
    plugins:
      proxy-rewrite:
        regex_uri: ["^/api/identity/(.*)", "/$1"]

  - uri: /api/pipeline/*
    upstream:
      type: roundrobin
      nodes:
        "host.docker.internal:8083": 1
    plugins:
      proxy-rewrite:
        regex_uri: ["^/api/pipeline/(.*)", "/$1"]

  - uri: /api/specs/*
    upstream:
      type: roundrobin
      nodes:
        "host.docker.internal:8084": 1
    plugins:
      proxy-rewrite:
        regex_uri: ["^/api/specs/(.*)", "/$1"]

  - uri: /api/bot/*
    upstream:
      type: roundrobin
      nodes:
        "host.docker.internal:8085": 1
    plugins:
      proxy-rewrite:
        regex_uri: ["^/api/bot/(.*)", "/$1"]

  - uri: /ws/*
    upstream:
      type: roundrobin
      nodes:
        "host.docker.internal:3001": 1
    plugins:
      proxy-rewrite:
        regex_uri: ["^/ws/(.*)", "/$1"]
    enable_websocket: true

upstreams: []
#END
```

- [ ] **Step 2: 重启 APISIX 加载新路由**

```bash
docker compose restart apisix
```

- [ ] **Step 3: 验证路由（需要先启动对应服务才能 200，此处验证路由注册即可）**

```bash
curl -s http://localhost:9080/apisix/status
```
Expected: 返回 APISIX 状态正常

- [ ] **Step 4: Commit**

```bash
git add docker/apisix/apisix.yaml
git commit -m "chore(m0): add APISIX routes for all services"
```

---

### Task 11: 端到端冒烟测试

- [ ] **Step 1: 确保 Docker Compose 全部启动**

```bash
docker compose up -d
docker compose ps
```
Expected: 全部 7 个容器 running

- [ ] **Step 2: 启动 forge-engine 验证完整链路**

```bash
cd forge-engine && mvn spring-boot:run &
```

等待启动后：
```bash
# 直接访问
curl -s http://localhost:8081/actuator/health
# 通过 APISIX 网关访问
curl -s http://localhost:9080/api/engine/actuator/health
```
Expected: 两者都返回 `{"status":"UP"}`

- [ ] **Step 3: 停止服务，清理**

```bash
# 停止 spring-boot:run 进程
kill %1
```

- [ ] **Step 4: 在根目录全量编译确认**

```bash
mvn clean compile -q
cd forge-portal && npm run build && cd ..
cd forge-beacon && npm run build && cd ..
```
Expected: 全部通过

---

## M0 完成标准

- [ ] 根 parent pom + 6 个 Java 子模块 + 1 个 Vue 项目 + 1 个 Node.js 项目全部编译通过
- [ ] Docker Compose 一键启动 MySQL / Redis / Kafka / Nacos / ES / APISIX / etcd
- [ ] Nacos 有 dev/staging/prod 三个命名空间 + 各服务 dev 配置
- [ ] APISIX 路由配置完成，请求可通过网关转发到后端服务
- [ ] Kafka topics 创建完成（forge.task.dispatch / forge.task.result / forge.task.event）
- [ ] README.md 和 CLAUDE.md 更新完成
- [ ] 所有变更已 commit
