package com.shulex.forge.identity.infrastructure.entity;

import com.baomidou.mybatisplus.annotation.*;
import lombok.Data;
import java.time.LocalDateTime;

@Data
@TableName("identity_role")
public class RoleDO {
    @TableId(type = IdType.AUTO)
    private Long id;
    private Long tenantId;
    private String roleCode;
    private String roleName;
    @TableField(fill = FieldFill.INSERT)
    private LocalDateTime gmtCreate;
    @TableField(fill = FieldFill.INSERT_UPDATE)
    private LocalDateTime gmtModified;
}
