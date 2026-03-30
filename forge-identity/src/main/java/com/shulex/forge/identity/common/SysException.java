package com.shulex.forge.identity.common;

import lombok.Getter;

@Getter
public class SysException extends RuntimeException {
    private final ErrorCode errorCode;

    public SysException(ErrorCode errorCode, Throwable cause) {
        super(errorCode.getMessage(), cause);
        this.errorCode = errorCode;
    }
}
