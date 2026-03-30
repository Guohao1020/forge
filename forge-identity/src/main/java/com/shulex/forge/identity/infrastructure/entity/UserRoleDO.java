package com.shulex.forge.identity.infrastructure.entity;

import com.baomidou.mybatisplus.annotation.*;
import lombok.Data;
import java.time.LocalDateTime;

@Data
@TableName("identity_user_role")
public class UserRoleDO {
    @TableId(type = IdType.AUTO)
    private Long id;
    private Long userId;
    private Long roleId;
    @TableField(fill = FieldFill.INSERT)
    private LocalDateTime gmtCreate;
}
