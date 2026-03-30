package com.shulex.forge.identity.common;

import lombok.extern.slf4j.Slf4j;
import org.springframework.http.ResponseEntity;
import org.springframework.web.bind.MethodArgumentNotValidException;
import org.springframework.web.bind.annotation.ExceptionHandler;
import org.springframework.web.bind.annotation.RestControllerAdvice;

@Slf4j
@RestControllerAdvice
public class GlobalExceptionHandler {

    @ExceptionHandler(BizException.class)
    public ResponseEntity<Result<Void>> handleBiz(BizException e) {
        log.warn("业务异常: {}", e.getMessage());
        int httpStatus = e.getErrorCode().getCode() / 100;
        return ResponseEntity.status(httpStatus)
                .body(Result.fail(e.getErrorCode().getCode(), e.getMessage()));
    }

    @ExceptionHandler(MethodArgumentNotValidException.class)
    public ResponseEntity<Result<Void>> handleValidation(MethodArgumentNotValidException e) {
        String message = e.getBindingResult().getFieldErrors().stream()
                .map(f -> f.getField() + " " + f.getDefaultMessage())
                .reduce((a, b) -> a + "; " + b)
                .orElse("参数错误");
        log.warn("参数校验失败: {}", message);
        return ResponseEntity.badRequest()
                .body(Result.fail(ErrorCode.INVALID_PARAM.getCode(), message));
    }

    @ExceptionHandler(SysException.class)
    public ResponseEntity<Result<Void>> handleSys(SysException e) {
        log.error("系统异常: {}", e.getMessage(), e.getCause());
        return ResponseEntity.internalServerError()
                .body(Result.fail(e.getErrorCode().getCode(), e.getMessage()));
    }

    @ExceptionHandler(Exception.class)
    public ResponseEntity<Result<Void>> handleUnknown(Exception e) {
        log.error("系统异常", e);
        return ResponseEntity.internalServerError()
                .body(Result.fail(ErrorCode.INTERNAL_ERROR.getCode(), ErrorCode.INTERNAL_ERROR.getMessage()));
    }
}
