package com.shulex.forge.pipeline.devops.model;

import lombok.AllArgsConstructor;
import lombok.Builder;
import lombok.Data;
import lombok.NoArgsConstructor;

@Data
@Builder
@NoArgsConstructor
@AllArgsConstructor
public class QualityGateResult {
    private boolean compilePassed;
    private boolean testPassed;
    private boolean reviewPassed;
    private boolean overallPassed;
    private String failureReason;
}
