package com.shulex.forge.pipeline.adapter.model;
import lombok.Builder;
import lombok.Data;
@Data
@Builder
public class MergeRequestInfo {
    private Long id;
    private String title;
    private String description;
    private String sourceBranch;
    private String targetBranch;
    private String state;
    private String url;
}
