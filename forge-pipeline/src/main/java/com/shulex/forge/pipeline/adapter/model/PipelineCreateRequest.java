package com.shulex.forge.pipeline.adapter.model;
import lombok.Builder;
import lombok.Data;
@Data
@Builder
public class PipelineCreateRequest {
    private String name;
    private String repoUrl;
    private String branch;
    private String yamlContent;
}
