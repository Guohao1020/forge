package com.shulex.forge.engine.entrance.vo;

import lombok.Builder;
import lombok.Data;
import java.time.LocalDateTime;

@Data
@Builder
public class TaskVO {
    private Long id;
    private Long tenantId;
    private Long userId;
    private String requirement;
    private String taskType;
    private String status;
    private String riskLevel;
    private String repoId;
    private String branchName;
    private Long mrId;
    private Integer reviewScore;
    private Long totalInputTokens;
    private Long totalOutputTokens;
    private LocalDateTime gmtCreate;
}
