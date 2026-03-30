package com.shulex.forge.pipeline.adapter.model;
import lombok.Builder;
import lombok.Data;
@Data
@Builder
public class FileContent {
    private String path;
    private String content;
    private String encoding;
    private String sha;
}
