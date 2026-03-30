-- V5: Seed scaffold templates (Java microservice)

INSERT INTO spec_scaffold_template (name, description, tech_stack, template_content, is_active) VALUES

('java-microservice', 'Java Spring Boot 微服务骨架模板，包含标准分层结构、公共基础设施和配置文件', 'Java 17, Spring Boot 3.2, MyBatis Plus 3.5, Maven',
'{
  "groupId": "{{groupId}}",
  "artifactId": "{{artifactId}}",
  "version": "1.0.0-SNAPSHOT",
  "files": [
    {
      "path": "pom.xml",
      "content": "<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n<project xmlns=\"http://maven.apache.org/POM/4.0.0\"\n         xmlns:xsi=\"http://www.w3.org/2001/XMLSchema-instance\"\n         xsi:schemaLocation=\"http://maven.apache.org/POM/4.0.0 http://maven.apache.org/xsd/maven-4.0.0.xsd\">\n    <modelVersion>4.0.0</modelVersion>\n\n    <groupId>{{groupId}}</groupId>\n    <artifactId>{{artifactId}}</artifactId>\n    <version>1.0.0-SNAPSHOT</version>\n    <packaging>jar</packaging>\n\n    <parent>\n        <groupId>org.springframework.boot</groupId>\n        <artifactId>spring-boot-starter-parent</artifactId>\n        <version>3.2.4</version>\n        <relativePath/>\n    </parent>\n\n    <properties>\n        <java.version>17</java.version>\n        <mybatis-plus.version>3.5.5</mybatis-plus.version>\n    </properties>\n\n    <dependencies>\n        <dependency>\n            <groupId>org.springframework.boot</groupId>\n            <artifactId>spring-boot-starter-web</artifactId>\n        </dependency>\n        <dependency>\n            <groupId>com.baomidou</groupId>\n            <artifactId>mybatis-plus-boot-starter</artifactId>\n            <version>${mybatis-plus.version}</version>\n        </dependency>\n        <dependency>\n            <groupId>com.mysql</groupId>\n            <artifactId>mysql-connector-j</artifactId>\n            <scope>runtime</scope>\n        </dependency>\n        <dependency>\n            <groupId>org.flywaydb</groupId>\n            <artifactId>flyway-mysql</artifactId>\n        </dependency>\n        <dependency>\n            <groupId>org.projectlombok</groupId>\n            <artifactId>lombok</artifactId>\n            <optional>true</optional>\n        </dependency>\n        <dependency>\n            <groupId>org.springframework.boot</groupId>\n            <artifactId>spring-boot-starter-test</artifactId>\n            <scope>test</scope>\n        </dependency>\n    </dependencies>\n\n    <build>\n        <plugins>\n            <plugin>\n                <groupId>org.springframework.boot</groupId>\n                <artifactId>spring-boot-maven-plugin</artifactId>\n            </plugin>\n        </plugins>\n    </build>\n</project>"
    },
    {
      "path": "src/main/java/{{packagePath}}/{{ServiceName}}Application.java",
      "content": "package {{packageName}};\n\nimport org.springframework.boot.SpringApplication;\nimport org.springframework.boot.autoconfigure.SpringBootApplication;\n\n@SpringBootApplication\npublic class {{ServiceName}}Application {\n\n    public static void main(String[] args) {\n        SpringApplication.run({{ServiceName}}Application.class, args);\n    }\n}"
    },
    {
      "path": "src/main/java/{{packagePath}}/common/Result.java",
      "content": "package {{packageName}}.common;\n\nimport lombok.Data;\n\n@Data\npublic class Result<T> {\n    private int code;\n    private String message;\n    private T data;\n\n    public static <T> Result<T> ok(T data) {\n        Result<T> result = new Result<>();\n        result.setCode(0);\n        result.setMessage(\"success\");\n        result.setData(data);\n        return result;\n    }\n\n    public static <T> Result<T> fail(int code, String message) {\n        Result<T> result = new Result<>();\n        result.setCode(code);\n        result.setMessage(message);\n        return result;\n    }\n}"
    },
    {
      "path": "src/main/java/{{packagePath}}/common/BizException.java",
      "content": "package {{packageName}}.common;\n\npublic class BizException extends RuntimeException {\n    private final int code;\n\n    public BizException(ErrorCode errorCode) {\n        super(errorCode.getMessage());\n        this.code = errorCode.getCode();\n    }\n\n    public int getCode() {\n        return code;\n    }\n}"
    },
    {
      "path": "src/main/java/{{packagePath}}/common/ErrorCode.java",
      "content": "package {{packageName}}.common;\n\npublic enum ErrorCode {\n    NOT_FOUND(40400, \"Resource not found\"),\n    BAD_REQUEST(40000, \"Bad request\"),\n    INTERNAL_ERROR(50000, \"Internal server error\");\n\n    private final int code;\n    private final String message;\n\n    ErrorCode(int code, String message) {\n        this.code = code;\n        this.message = message;\n    }\n\n    public int getCode() { return code; }\n    public String getMessage() { return message; }\n}"
    },
    {
      "path": "src/main/java/{{packagePath}}/common/GlobalExceptionHandler.java",
      "content": "package {{packageName}}.common;\n\nimport org.slf4j.Logger;\nimport org.slf4j.LoggerFactory;\nimport org.springframework.web.bind.annotation.ExceptionHandler;\nimport org.springframework.web.bind.annotation.RestControllerAdvice;\n\n@RestControllerAdvice\npublic class GlobalExceptionHandler {\n\n    private static final Logger log = LoggerFactory.getLogger(GlobalExceptionHandler.class);\n\n    @ExceptionHandler(BizException.class)\n    public Result<Void> handleBizException(BizException e) {\n        log.warn(\"BizException: code={}, message={}\", e.getCode(), e.getMessage());\n        return Result.fail(e.getCode(), e.getMessage());\n    }\n\n    @ExceptionHandler(Exception.class)\n    public Result<Void> handleException(Exception e) {\n        log.error(\"Unexpected error\", e);\n        return Result.fail(ErrorCode.INTERNAL_ERROR.getCode(), ErrorCode.INTERNAL_ERROR.getMessage());\n    }\n}"
    },
    {
      "path": "src/main/java/{{packagePath}}/infrastructure/entity/ExampleDO.java",
      "content": "package {{packageName}}.infrastructure.entity;\n\nimport com.baomidou.mybatisplus.annotation.*;\nimport lombok.Data;\nimport java.time.LocalDateTime;\n\n@Data\n@TableName(\"{{tableName}}\")\npublic class ExampleDO {\n    @TableId(type = IdType.AUTO)\n    private Long id;\n    private String name;\n    @TableField(fill = FieldFill.INSERT)\n    private LocalDateTime gmtCreate;\n    @TableField(fill = FieldFill.INSERT_UPDATE)\n    private LocalDateTime gmtModified;\n}"
    },
    {
      "path": "src/main/java/{{packagePath}}/infrastructure/mapper/ExampleMapper.java",
      "content": "package {{packageName}}.infrastructure.mapper;\n\nimport com.baomidou.mybatisplus.core.mapper.BaseMapper;\nimport {{packageName}}.infrastructure.entity.ExampleDO;\nimport org.apache.ibatis.annotations.Mapper;\n\n@Mapper\npublic interface ExampleMapper extends BaseMapper<ExampleDO> {\n}"
    },
    {
      "path": "src/main/java/{{packagePath}}/infrastructure/config/MybatisPlusConfig.java",
      "content": "package {{packageName}}.infrastructure.config;\n\nimport com.baomidou.mybatisplus.core.handlers.MetaObjectHandler;\nimport org.apache.ibatis.reflection.MetaObject;\nimport org.springframework.stereotype.Component;\nimport java.time.LocalDateTime;\n\n@Component\npublic class MybatisPlusConfig implements MetaObjectHandler {\n\n    @Override\n    public void insertFill(MetaObject metaObject) {\n        this.strictInsertFill(metaObject, \"gmtCreate\", LocalDateTime.class, LocalDateTime.now());\n        this.strictInsertFill(metaObject, \"gmtModified\", LocalDateTime.class, LocalDateTime.now());\n    }\n\n    @Override\n    public void updateFill(MetaObject metaObject) {\n        this.strictUpdateFill(metaObject, \"gmtModified\", LocalDateTime.class, LocalDateTime.now());\n    }\n}"
    },
    {
      "path": "src/main/java/{{packagePath}}/service/ExampleService.java",
      "content": "package {{packageName}}.service;\n\nimport com.baomidou.mybatisplus.core.conditions.query.LambdaQueryWrapper;\nimport {{packageName}}.common.BizException;\nimport {{packageName}}.common.ErrorCode;\nimport {{packageName}}.infrastructure.entity.ExampleDO;\nimport {{packageName}}.infrastructure.mapper.ExampleMapper;\nimport org.slf4j.Logger;\nimport org.slf4j.LoggerFactory;\nimport org.springframework.stereotype.Service;\nimport java.util.List;\n\n@Service\npublic class ExampleService {\n\n    private static final Logger log = LoggerFactory.getLogger(ExampleService.class);\n\n    private final ExampleMapper exampleMapper;\n\n    public ExampleService(ExampleMapper exampleMapper) {\n        this.exampleMapper = exampleMapper;\n    }\n\n    public ExampleDO getById(Long id) {\n        ExampleDO entity = exampleMapper.selectById(id);\n        if (entity == null) {\n            throw new BizException(ErrorCode.NOT_FOUND);\n        }\n        return entity;\n    }\n\n    public List<ExampleDO> listAll() {\n        return exampleMapper.selectList(new LambdaQueryWrapper<>());\n    }\n}"
    },
    {
      "path": "src/main/java/{{packagePath}}/entrance/vo/ExampleVO.java",
      "content": "package {{packageName}}.entrance.vo;\n\nimport lombok.Data;\n\n@Data\npublic class ExampleVO {\n    private Long id;\n    private String name;\n}"
    },
    {
      "path": "src/main/java/{{packagePath}}/entrance/controller/ExampleController.java",
      "content": "package {{packageName}}.entrance.controller;\n\nimport {{packageName}}.common.Result;\nimport {{packageName}}.entrance.vo.ExampleVO;\nimport {{packageName}}.infrastructure.entity.ExampleDO;\nimport {{packageName}}.service.ExampleService;\nimport org.springframework.web.bind.annotation.*;\nimport java.util.List;\n\n@RestController\n@RequestMapping(\"/api/examples\")\npublic class ExampleController {\n\n    private final ExampleService exampleService;\n\n    public ExampleController(ExampleService exampleService) {\n        this.exampleService = exampleService;\n    }\n\n    @GetMapping\n    public Result<List<ExampleVO>> listAll() {\n        List<ExampleDO> list = exampleService.listAll();\n        return Result.ok(list.stream().map(this::toVO).toList());\n    }\n\n    @GetMapping(\"/{id}\")\n    public Result<ExampleVO> getById(@PathVariable(\"id\") Long id) {\n        ExampleDO entity = exampleService.getById(id);\n        return Result.ok(toVO(entity));\n    }\n\n    private ExampleVO toVO(ExampleDO entity) {\n        ExampleVO vo = new ExampleVO();\n        vo.setId(entity.getId());\n        vo.setName(entity.getName());\n        return vo;\n    }\n}"
    },
    {
      "path": "src/main/resources/application.yml",
      "content": "server:\n  port: {{port}}\n\nspring:\n  application:\n    name: {{artifactId}}\n  datasource:\n    url: jdbc:mysql://localhost:3306/{{dbName}}?useUnicode=true&characterEncoding=utf8&useSSL=false&serverTimezone=Asia/Shanghai\n    username: ${DB_USERNAME:root}\n    password: ${DB_PASSWORD:root}\n    driver-class-name: com.mysql.cj.jdbc.Driver\n  flyway:\n    locations: classpath:db/migration\n    baseline-on-migrate: true\n\nmybatis-plus:\n  configuration:\n    map-underscore-to-camel-case: true\n    log-impl: org.apache.ibatis.logging.slf4j.Slf4jImpl\n  global-config:\n    db-config:\n      id-type: auto\n\nlogging:\n  level:\n    {{packageName}}: INFO"
    },
    {
      "path": "src/main/resources/db/migration/V1__init_tables.sql",
      "content": "-- V1: Initialize tables for {{artifactId}}\n\nCREATE TABLE IF NOT EXISTS {{tableName}} (\n    id          BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT ''主键'',\n    name        VARCHAR(255)    NOT NULL DEFAULT '''' COMMENT ''名称'',\n    gmt_create  DATETIME        NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT ''创建时间'',\n    gmt_modified DATETIME       NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT ''修改时间'',\n    PRIMARY KEY (id),\n    KEY idx_gmt_create (gmt_create)\n) ENGINE = InnoDB\n  DEFAULT CHARSET = utf8mb4\n  COMMENT = ''示例表'';"
    }
  ],
  "directories": [
    "src/main/java/{{packagePath}}/common",
    "src/main/java/{{packagePath}}/entrance/controller",
    "src/main/java/{{packagePath}}/entrance/vo",
    "src/main/java/{{packagePath}}/service",
    "src/main/java/{{packagePath}}/infrastructure/entity",
    "src/main/java/{{packagePath}}/infrastructure/mapper",
    "src/main/java/{{packagePath}}/infrastructure/config",
    "src/main/resources/db/migration",
    "src/test/java/{{packagePath}}"
  ],
  "placeholders": {
    "groupId": "com.example",
    "artifactId": "my-service",
    "packageName": "com.example.myservice",
    "packagePath": "com/example/myservice",
    "ServiceName": "MyService",
    "tableName": "example",
    "dbName": "my_service_db",
    "port": "8080"
  }
}',
1);
