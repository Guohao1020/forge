package com.shulex.forge.engine.entrance.vo;

import jakarta.validation.constraints.NotBlank;
import jakarta.validation.constraints.NotNull;
import lombok.Data;

@Data
public class CreateTaskRequest {
    @NotNull
    private Long tenantId;
    @NotNull
    private Long userId;
    @NotBlank
    private String requirement;
    private String taskType = "GENERATE";
    @NotBlank
    private String repoId;
}
