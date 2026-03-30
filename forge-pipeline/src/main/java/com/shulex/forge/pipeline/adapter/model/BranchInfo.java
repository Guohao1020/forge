package com.shulex.forge.pipeline.adapter.model;
import lombok.Builder;
import lombok.Data;
@Data
@Builder
public class BranchInfo {
    private String name;
    private String commitId;
    private boolean isProtected;
}
