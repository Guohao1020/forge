package com.shulex.forge.pipeline.adapter.model;
import lombok.Builder;
import lombok.Data;
@Data
@Builder
public class CommitFile {
    private String path;
    private String content;
    private String action;
}
