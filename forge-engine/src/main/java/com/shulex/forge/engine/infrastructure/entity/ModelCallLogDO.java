package com.shulex.forge.engine.infrastructure.entity;

import com.baomidou.mybatisplus.annotation.*;
import lombok.Data;
import java.time.LocalDateTime;

@Data
@TableName("engine_model_call_log")
public class ModelCallLogDO {
    @TableId(type = IdType.AUTO)
    private Long id;
    private Long taskId;
    private Long stepId;
    private String modelId;
    private String purpose;
    private Long inputTokens;
    private Long outputTokens;
    private Long latencyMs;
    private Integer isFallback;
    @TableField(fill = FieldFill.INSERT)
    private LocalDateTime gmtCreate;
}
