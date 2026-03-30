package com.shulex.forge.identity.infrastructure.entity;

import com.baomidou.mybatisplus.annotation.*;
import lombok.Data;
import java.time.LocalDateTime;

@Data
@TableName("identity_api_token")
public class ApiTokenDO {
    @TableId(type = IdType.AUTO)
    private Long id;
    private Long tenantId;
    private Long userId;
    private String tokenName;
    private String tokenHash;
    private String tokenPrefix;
    private LocalDateTime expiresAt;
    private Integer status;
    @TableField(fill = FieldFill.INSERT)
    private LocalDateTime gmtCreate;
    @TableField(fill = FieldFill.INSERT_UPDATE)
    private LocalDateTime gmtModified;
}
