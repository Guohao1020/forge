package com.shulex.forge.engine.execution.ai;

import lombok.AllArgsConstructor;
import lombok.Data;
import lombok.NoArgsConstructor;

@Data
@NoArgsConstructor
@AllArgsConstructor
public class AiResponse {
    private String content;
    private long inputTokens;
    private long outputTokens;
    private String model;
    private String stopReason;
}
