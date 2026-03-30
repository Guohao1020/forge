package com.shulex.forge.pipeline.adapter.model;
import lombok.Builder;
import lombok.Data;
@Data
@Builder
public class DeploymentInfo {
    private String namespace;
    private String name;
    private String image;
    private Integer replicas;
    private Integer availableReplicas;
    private String status;
}
