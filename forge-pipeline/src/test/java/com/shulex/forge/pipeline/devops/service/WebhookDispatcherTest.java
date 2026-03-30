package com.shulex.forge.pipeline.devops.service;

import com.shulex.forge.pipeline.infrastructure.entity.PipelineExecutionDO;
import com.shulex.forge.pipeline.infrastructure.mapper.PipelineExecutionMapper;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;

import static org.assertj.core.api.Assertions.assertThat;
import static org.mockito.ArgumentMatchers.any;
import static org.mockito.Mockito.*;

class WebhookDispatcherTest {

    private WebhookDispatcher webhookDispatcher;
    private PipelineExecutionMapper executionMapper;
    private PipelineTemplateService templateService;
    private EnvironmentService environmentService;
    private DeploymentService deploymentService;
    private QualityGateService qualityGateService;

    @BeforeEach
    void setUp() {
        executionMapper = mock(PipelineExecutionMapper.class);
        templateService = mock(PipelineTemplateService.class);
        environmentService = mock(EnvironmentService.class);
        deploymentService = mock(DeploymentService.class);
        qualityGateService = mock(QualityGateService.class);
        webhookDispatcher = new WebhookDispatcher(executionMapper, templateService,
                environmentService, deploymentService, qualityGateService);
    }

    @Test
    void onPush_createsExecutionRecord() {
        when(executionMapper.insert(any())).thenReturn(1);

        webhookDispatcher.onPush(1L, "repo-123", "main", "WEBHOOK");

        verify(executionMapper).insert(any());
    }

    @Test
    void onPush_aisBranch_triggersTemporaryEnvironment() {
        when(executionMapper.insert(any())).thenReturn(1);

        webhookDispatcher.onPush(1L, "repo-123", "ai/feature-login", "AI");

        verify(environmentService).createTemporaryEnvironment(eq(1L), eq("repo-123"), eq("ai/feature-login"), any());
    }
}
