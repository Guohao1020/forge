package com.shulex.forge.specs.infrastructure.entity;

import com.baomidou.mybatisplus.annotation.*;
import lombok.Data;
import java.time.LocalDateTime;

@Data
@TableName("spec_review_rule")
public class ReviewRuleDO {
    @TableId(type = IdType.AUTO)
    private Long id;
    private String category;
    private String ruleKey;
    private String name;
    private String description;
    private String severity;
    private Boolean isEnabled;
    @TableField(fill = FieldFill.INSERT)
    private LocalDateTime gmtCreate;
    @TableField(fill = FieldFill.INSERT_UPDATE)
    private LocalDateTime gmtModified;
}
