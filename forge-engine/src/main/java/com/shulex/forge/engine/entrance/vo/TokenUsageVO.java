package com.shulex.forge.engine.entrance.vo;

import lombok.Builder;
import lombok.Data;

@Data
@Builder
public class TokenUsageVO {
    private Long taskId;
    private Long totalInputTokens;
    private Long totalOutputTokens;
}
