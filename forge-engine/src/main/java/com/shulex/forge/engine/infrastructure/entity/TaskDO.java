package com.shulex.forge.engine.infrastructure.entity;

import com.baomidou.mybatisplus.annotation.*;
import lombok.Data;
import java.time.LocalDateTime;

@Data
@TableName("engine_task")
public class TaskDO {
    @TableId(type = IdType.AUTO)
    private Long id;
    private Long tenantId;
    private Long userId;
    private String requirement;
    private String taskType;
    private String status;
    private String riskLevel;
    private String repoId;
    private String branchName;
    private Long mrId;
    private Integer reviewScore;
    private Long totalInputTokens;
    private Long totalOutputTokens;
    private String errorMessage;
    @TableField(fill = FieldFill.INSERT)
    private LocalDateTime gmtCreate;
    @TableField(fill = FieldFill.INSERT_UPDATE)
    private LocalDateTime gmtModified;
}
