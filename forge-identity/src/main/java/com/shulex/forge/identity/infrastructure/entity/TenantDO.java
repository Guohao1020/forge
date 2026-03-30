package com.shulex.forge.identity.infrastructure.entity;

import com.baomidou.mybatisplus.annotation.*;
import lombok.Data;
import java.time.LocalDateTime;

@Data
@TableName("identity_tenant")
public class TenantDO {
    @TableId(type = IdType.AUTO)
    private Long id;
    private String name;
    private Integer status;
    @TableField(fill = FieldFill.INSERT)
    private LocalDateTime gmtCreate;
    @TableField(fill = FieldFill.INSERT_UPDATE)
    private LocalDateTime gmtModified;
}
