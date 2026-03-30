package com.shulex.forge.engine.entrance.vo;

import lombok.Builder;
import lombok.Data;

@Data
@Builder
public class TaskStepVO {
    private Long id;
    private String stepType;
    private Integer stepOrder;
    private String status;
    private Long inputTokens;
    private Long outputTokens;
    private Integer retryCount;
    private String outputSnapshot;
    private String errorMessage;
}
