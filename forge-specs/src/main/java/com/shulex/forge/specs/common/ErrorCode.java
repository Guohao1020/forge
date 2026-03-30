package com.shulex.forge.specs.common;

import lombok.Getter;
import lombok.AllArgsConstructor;

@Getter
@AllArgsConstructor
public enum ErrorCode {
    NOT_FOUND(40400, "资源不存在"),
    INVALID_PARAM(40000, "参数错误"),
    INTERNAL_ERROR(50000, "系统内部错误");

    private final int code;
    private final String message;
}
