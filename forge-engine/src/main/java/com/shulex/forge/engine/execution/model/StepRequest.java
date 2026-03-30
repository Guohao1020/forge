package com.shulex.forge.engine.execution.model;

import lombok.AllArgsConstructor;
import lombok.Data;
import lombok.NoArgsConstructor;

@Data
@NoArgsConstructor
@AllArgsConstructor
public class StepRequest {
    private Long taskId;
    private Long stepId;
    private String stepType;
    private String adapterType;
    private String repoId;
    private String branchName;
    private String requirement;
    private String inputData;
}
