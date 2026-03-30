package com.shulex.forge.engine.common;

import lombok.Getter;
import lombok.AllArgsConstructor;

@Getter
@AllArgsConstructor
public enum ErrorCode {
    TASK_NOT_FOUND(40400, "任务不存在"),
    TASK_INVALID_STATUS(40001, "任务状态不允许此操作"),
    STEP_NOT_FOUND(40401, "步骤不存在"),
    KILL_SWITCH_ACTIVE(40301, "紧急停止已激活，无法创建新任务"),
    INVALID_PARAM(40000, "参数错误"),
    AI_CALL_FAILED(50001, "AI 模型调用失败"),
    CODE_COMMIT_FAILED(50002, "代码提交失败"),
    CONTEXT_BUILD_FAILED(50003, "上下文构建失败"),
    INTERNAL_ERROR(50000, "系统内部错误");

    private final int code;
    private final String message;
}
