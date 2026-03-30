package com.shulex.forge.pipeline.adapter.model;
import lombok.Builder;
import lombok.Data;
import java.time.LocalDateTime;
@Data
@Builder
public class PodInfo {
    private String namespace;
    private String name;
    private String phase;
    private String nodeName;
    private LocalDateTime startTime;
}
