package com.shulex.forge.specs.infrastructure.entity;

import com.baomidou.mybatisplus.annotation.*;
import lombok.Data;
import java.time.LocalDateTime;

@Data
@TableName("spec_standard")
public class StandardDO {
    @TableId(type = IdType.AUTO)
    private Long id;
    private String category;
    private String title;
    private String content;
    private String scopeLevel;
    private String scopeId;
    private Integer sortOrder;
    private Boolean isEnabled;
    @TableField(fill = FieldFill.INSERT)
    private LocalDateTime gmtCreate;
    @TableField(fill = FieldFill.INSERT_UPDATE)
    private LocalDateTime gmtModified;
}
