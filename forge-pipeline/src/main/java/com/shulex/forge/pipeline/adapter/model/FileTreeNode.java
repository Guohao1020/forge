package com.shulex.forge.pipeline.adapter.model;
import lombok.Builder;
import lombok.Data;
@Data
@Builder
public class FileTreeNode {
    private String path;
    private String name;
    private String type;
    private Long size;
}
