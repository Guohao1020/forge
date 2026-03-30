package com.shulex.forge.identity.common;

import lombok.Getter;
import lombok.AllArgsConstructor;

@Getter
@AllArgsConstructor
public enum ErrorCode {
    INVALID_CREDENTIALS(40100, "用户名或密码错误"),
    TOKEN_EXPIRED(40101, "Token 已过期"),
    TOKEN_INVALID(40102, "Token 无效"),
    TOKEN_REVOKED(40103, "Token 已吊销"),
    UNAUTHORIZED(40104, "未认证"),
    FORBIDDEN(40300, "无权限"),
    USER_NOT_FOUND(40400, "用户不存在"),
    USER_DISABLED(40301, "用户已禁用"),
    USER_EXISTS(40901, "用户名已存在"),
    TENANT_NOT_FOUND(40402, "租户不存在"),
    INVALID_PARAM(40000, "参数错误"),
    INTERNAL_ERROR(50000, "系统内部错误");

    private final int code;
    private final String message;
}
