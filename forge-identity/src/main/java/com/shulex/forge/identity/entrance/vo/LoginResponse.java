package com.shulex.forge.identity.entrance.vo;

import lombok.Builder;
import lombok.Data;

@Data
@Builder
public class LoginResponse {
    private String accessToken;
    private String refreshToken;
    private Long userId;
    private String username;
    private java.util.List<String> roles;
}
