package com.shulex.forge.identity.entrance.vo;

import lombok.Builder;
import lombok.Data;

@Data
@Builder
public class UserVO {
    private Long id;
    private Long tenantId;
    private String username;
    private String nickname;
    private String email;
    private Integer status;
}
