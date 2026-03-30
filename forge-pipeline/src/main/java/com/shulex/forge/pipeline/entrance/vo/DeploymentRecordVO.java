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
public class DeploymentRecordVO {
    private Long id;
    private String namespace;
    private String deploymentName;
    private String image;
    private String status;
    private String branch;
    private LocalDateTime gmtCreate;
}
