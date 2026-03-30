package com.shulex.forge.pipeline.infrastructure.entity;

import com.baomidou.mybatisplus.annotation.*;
import lombok.Data;
import java.time.LocalDateTime;

@Data
@TableName("deployment_record")
public class DeploymentRecordDO {
    @TableId(type = IdType.AUTO)
    private Long id;
    private Long tenantId;
    private Long environmentId;
    private String repoId;
    private String branch;
    private String image;
    private String namespace;
    private String deploymentName;
    private Integer replicas;
    private String status;
    private String errorMessage;
    private Long pipelineExecutionId;
    @TableField(fill = FieldFill.INSERT)
    private LocalDateTime gmtCreate;
    @TableField(fill = FieldFill.INSERT_UPDATE)
    private LocalDateTime gmtModified;
}
