package com.shulex.forge.pipeline.infrastructure.entity;

import com.baomidou.mybatisplus.annotation.*;
import lombok.Data;
import java.time.LocalDateTime;

@Data
@TableName("pipeline_execution")
public class PipelineExecutionDO {
    @TableId(type = IdType.AUTO)
    private Long id;
    private Long tenantId;
    private String repoId;
    private String branch;
    private String pipelineId;
    private String runId;
    private String projectType;
    private String status;
    private Integer compilePassed;
    private Integer testPassed;
    private Integer reviewPassed;
    private Integer qualityGatePassed;
    private String logUrl;
    private String errorMessage;
    private String triggerType;
    private Long taskId;
    @TableField(fill = FieldFill.INSERT)
    private LocalDateTime gmtCreate;
    @TableField(fill = FieldFill.INSERT_UPDATE)
    private LocalDateTime gmtModified;
}
