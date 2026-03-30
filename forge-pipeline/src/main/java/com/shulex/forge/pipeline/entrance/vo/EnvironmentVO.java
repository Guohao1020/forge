package com.shulex.forge.pipeline.entrance.vo;

import lombok.AllArgsConstructor;
import lombok.Builder;
import lombok.Data;
import lombok.NoArgsConstructor;
import java.time.LocalDateTime;

@Data
@Builder
@NoArgsConstructor
@AllArgsConstructor
public class EnvironmentVO {
    private Long id;
    private String name;
    private String envType;
    private String namespace;
    private String boundBranch;
    private String status;
    private LocalDateTime autoDestroyAt;
    private LocalDateTime gmtCreate;
}
