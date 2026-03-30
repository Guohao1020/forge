package com.shulex.forge.identity.entrance.vo;

import jakarta.validation.constraints.NotBlank;
import jakarta.validation.constraints.NotNull;
import lombok.Data;

@Data
public class CreateUserRequest {
    @NotNull
    private Long tenantId;
    @NotBlank
    private String username;
    @NotBlank
    private String password;
    private String nickname;
    private String roleCode;
}
