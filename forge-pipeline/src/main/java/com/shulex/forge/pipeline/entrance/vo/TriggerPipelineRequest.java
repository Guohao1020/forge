package com.shulex.forge.pipeline.entrance.vo;

import jakarta.validation.constraints.NotBlank;
import jakarta.validation.constraints.NotNull;
import lombok.Data;

@Data
public class TriggerPipelineRequest {
    @NotNull
    private Long tenantId;
    @NotBlank
    private String repoId;
    @NotBlank
    private String branch;
    private String projectType;
}
