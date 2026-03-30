package com.shulex.forge.pipeline.devops.service;

import com.shulex.forge.pipeline.devops.model.QualityGateResult;
import com.shulex.forge.pipeline.infrastructure.entity.PipelineExecutionDO;
import com.shulex.forge.pipeline.infrastructure.mapper.PipelineExecutionMapper;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;

import static org.assertj.core.api.Assertions.assertThat;
import static org.mockito.Mockito.*;

class QualityGateServiceTest {

    private QualityGateService qualityGateService;
    private PipelineExecutionMapper executionMapper;

    @BeforeEach
    void setUp() {
        executionMapper = mock(PipelineExecutionMapper.class);
        qualityGateService = new QualityGateService(executionMapper);
    }

    @Test
    void evaluate_allPassed() {
        QualityGateResult result = qualityGateService.evaluate(true, true, true);
        assertThat(result.isOverallPassed()).isTrue();
    }

    @Test
    void evaluate_compileFailed() {
        QualityGateResult result = qualityGateService.evaluate(false, true, true);
        assertThat(result.isOverallPassed()).isFalse();
        assertThat(result.getFailureReason()).contains("编译");
    }

    @Test
    void evaluate_testFailed() {
        QualityGateResult result = qualityGateService.evaluate(true, false, true);
        assertThat(result.isOverallPassed()).isFalse();
        assertThat(result.getFailureReason()).contains("测试");
    }

    @Test
    void updateExecution_savesGateResult() {
        PipelineExecutionDO exec = new PipelineExecutionDO();
        exec.setId(1L);
        when(executionMapper.selectById(1L)).thenReturn(exec);
        when(executionMapper.updateById(any())).thenReturn(1);

        QualityGateResult result = QualityGateResult.builder()
                .compilePassed(true).testPassed(true).reviewPassed(true).overallPassed(true).build();

        qualityGateService.updateExecution(1L, result);
        verify(executionMapper).updateById(any());
    }
}
