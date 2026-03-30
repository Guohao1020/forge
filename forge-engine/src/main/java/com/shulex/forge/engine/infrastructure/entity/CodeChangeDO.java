package com.shulex.forge.engine.infrastructure.entity;

import com.baomidou.mybatisplus.annotation.*;
import lombok.Data;
import java.time.LocalDateTime;

@Data
@TableName("engine_code_change")
public class CodeChangeDO {
    @TableId(type = IdType.AUTO)
    private Long id;
    private Long taskId;
    private String repoId;
    private String branchName;
    private String commitHash;
    private Integer fileCount;
    private Integer reviewScore;
    private Long mrId;
    private String mrStatus;
    @TableField(fill = FieldFill.INSERT)
    private LocalDateTime gmtCreate;
    @TableField(fill = FieldFill.INSERT_UPDATE)
    private LocalDateTime gmtModified;
}
