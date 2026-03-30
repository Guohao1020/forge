package com.shulex.forge.pipeline.entrance.vo;

import lombok.AllArgsConstructor;
import lombok.Builder;
import lombok.Data;
import lombok.NoArgsConstructor;
import java.time.LocalDateTime;

@Data
@Builder
@NoArgsConstructor
@AllArgsConstructor
public class PipelineExecutionVO {
    private Long id;
    private String repoId;
    private String branch;
    private String projectType;
    private String status;
    private Boolean compilePassed;
    private Boolean testPassed;
    private Boolean reviewPassed;
    private Boolean qualityGatePassed;
    private String triggerType;
    private LocalDateTime gmtCreate;
}
