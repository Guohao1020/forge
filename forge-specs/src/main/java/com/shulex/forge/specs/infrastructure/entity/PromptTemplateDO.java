package com.shulex.forge.specs.infrastructure.entity;

import com.baomidou.mybatisplus.annotation.*;
import lombok.Data;
import java.time.LocalDateTime;

@Data
@TableName("spec_prompt_template")
public class PromptTemplateDO {
    @TableId(type = IdType.AUTO)
    private Long id;
    private String templateKey;
    private String name;
    private String description;
    private String systemPrompt;
    private String standardsInjection;
    private Integer version;
    private Boolean isActive;
    @TableField(fill = FieldFill.INSERT)
    private LocalDateTime gmtCreate;
    @TableField(fill = FieldFill.INSERT_UPDATE)
    private LocalDateTime gmtModified;
}
