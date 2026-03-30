package com.shulex.forge.pipeline.entrance.vo;

import jakarta.validation.constraints.NotBlank;
import jakarta.validation.constraints.NotNull;
import lombok.Data;

@Data
public class DeployRequest {
    @NotNull
    private Long tenantId;
    @NotBlank
    private String namespace;
    @NotBlank
    private String deploymentName;
    @NotBlank
    private String image;
    @NotBlank
    private String repoId;
    @NotBlank
    private String branch;
    private int replicas = 1;
}
