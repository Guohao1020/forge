package com.shulex.forge.pipeline.adapter.model;
import lombok.Builder;
import lombok.Data;
@Data
@Builder
public class PipelineInfo {
    private String id;
    private String name;
    private String status;
}
