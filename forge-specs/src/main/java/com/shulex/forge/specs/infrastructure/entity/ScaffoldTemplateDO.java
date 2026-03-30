package com.shulex.forge.specs.infrastructure.entity;

import com.baomidou.mybatisplus.annotation.*;
import lombok.Data;
import java.time.LocalDateTime;

@Data
@TableName("spec_scaffold_template")
public class ScaffoldTemplateDO {
    @TableId(type = IdType.AUTO)
    private Long id;
    private String name;
    private String description;
    private String techStack;
    private String templateContent;
    private Boolean isActive;
    @TableField(fill = FieldFill.INSERT)
    private LocalDateTime gmtCreate;
    @TableField(fill = FieldFill.INSERT_UPDATE)
    private LocalDateTime gmtModified;
}
