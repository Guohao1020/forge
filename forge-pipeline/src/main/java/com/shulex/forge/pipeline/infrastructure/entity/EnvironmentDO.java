package com.shulex.forge.pipeline.infrastructure.entity;

import com.baomidou.mybatisplus.annotation.*;
import lombok.Data;
import java.time.LocalDateTime;

@Data
@TableName("environment")
public class EnvironmentDO {
    @TableId(type = IdType.AUTO)
    private Long id;
    private Long tenantId;
    private String name;
    private String envType;
    private String namespace;
    private String boundBranch;
    private String status;
    private LocalDateTime autoDestroyAt;
    private String repoId;
    private Long taskId;
    @TableField(fill = FieldFill.INSERT)
    private LocalDateTime gmtCreate;
    @TableField(fill = FieldFill.INSERT_UPDATE)
    private LocalDateTime gmtModified;
}
