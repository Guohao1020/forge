package com.shulex.forge.pipeline.adapter.model;
import lombok.Builder;
import lombok.Data;
import java.time.LocalDateTime;
@Data
@Builder
public class PipelineRunInfo {
    private String runId;
    private String pipelineId;
    private String status;
    private String triggerType;
    private LocalDateTime startTime;
    private LocalDateTime endTime;
    private String logUrl;
}
