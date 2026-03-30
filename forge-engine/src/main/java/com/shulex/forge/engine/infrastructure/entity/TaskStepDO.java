package com.shulex.forge.engine.infrastructure.entity;

import com.baomidou.mybatisplus.annotation.*;
import lombok.Data;
import java.time.LocalDateTime;

@Data
@TableName("engine_task_step")
public class TaskStepDO {
    @TableId(type = IdType.AUTO)
    private Long id;
    private Long taskId;
    private String stepType;
    private Integer stepOrder;
    private String status;
    private String inputSnapshot;
    private String outputSnapshot;
    private Long inputTokens;
    private Long outputTokens;
    private Integer retryCount;
    private String errorMessage;
    @TableField(fill = FieldFill.INSERT)
    private LocalDateTime gmtCreate;
    @TableField(fill = FieldFill.INSERT_UPDATE)
    private LocalDateTime gmtModified;
}
