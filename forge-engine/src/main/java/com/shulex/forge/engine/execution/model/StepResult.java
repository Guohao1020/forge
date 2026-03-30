package com.shulex.forge.engine.execution.model;

import lombok.AllArgsConstructor;
import lombok.Data;
import lombok.NoArgsConstructor;

@Data
@NoArgsConstructor
@AllArgsConstructor
public class StepResult {
    private Long taskId;
    private Long stepId;
    private String stepType;
    private boolean success;
    private String outputData;
    private long inputTokens;
    private long outputTokens;
    private String errorMessage;
}
