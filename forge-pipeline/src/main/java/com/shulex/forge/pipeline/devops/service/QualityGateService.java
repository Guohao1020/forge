package com.shulex.forge.pipeline.devops.service;

import com.shulex.forge.pipeline.devops.model.QualityGateResult;
import com.shulex.forge.pipeline.infrastructure.entity.PipelineExecutionDO;
import com.shulex.forge.pipeline.infrastructure.mapper.PipelineExecutionMapper;
import lombok.extern.slf4j.Slf4j;
import org.springframework.stereotype.Service;

@Slf4j
@Service
public class QualityGateService {

    private final PipelineExecutionMapper executionMapper;

    public QualityGateService(PipelineExecutionMapper executionMapper) {
        this.executionMapper = executionMapper;
    }

    public QualityGateResult evaluate(boolean compilePassed, boolean testPassed, boolean reviewPassed) {
        StringBuilder failureReason = new StringBuilder();
        if (!compilePassed) failureReason.append("编译未通过; ");
        if (!testPassed) failureReason.append("测试未通过; ");
        if (!reviewPassed) failureReason.append("Review未通过; ");

        boolean overall = compilePassed && testPassed && reviewPassed;

        log.info("质量门禁评估: compile={}, test={}, review={}, overall={}",
                compilePassed, testPassed, reviewPassed, overall);

        return QualityGateResult.builder()
                .compilePassed(compilePassed)
                .testPassed(testPassed)
                .reviewPassed(reviewPassed)
                .overallPassed(overall)
                .failureReason(failureReason.length() > 0 ? failureReason.toString().trim() : null)
                .build();
    }

    public void updateExecution(Long executionId, QualityGateResult result) {
        PipelineExecutionDO exec = executionMapper.selectById(executionId);
        if (exec == null) return;
        exec.setCompilePassed(result.isCompilePassed() ? 1 : 0);
        exec.setTestPassed(result.isTestPassed() ? 1 : 0);
        exec.setReviewPassed(result.isReviewPassed() ? 1 : 0);
        exec.setQualityGatePassed(result.isOverallPassed() ? 1 : 0);
        executionMapper.updateById(exec);
    }
}
