package com.shulex.forge.identity.infrastructure.entity;

import com.baomidou.mybatisplus.annotation.*;
import lombok.Data;
import java.time.LocalDateTime;

@Data
@TableName("identity_user")
public class UserDO {
    @TableId(type = IdType.AUTO)
    private Long id;
    private Long tenantId;
    private String username;
    private String passwordHash;
    private String email;
    private String nickname;
    private Integer status;
    @TableField(fill = FieldFill.INSERT)
    private LocalDateTime gmtCreate;
    @TableField(fill = FieldFill.INSERT_UPDATE)
    private LocalDateTime gmtModified;
}
