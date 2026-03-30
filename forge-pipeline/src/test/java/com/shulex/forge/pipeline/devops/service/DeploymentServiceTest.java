package com.shulex.forge.pipeline.devops.service;

import com.shulex.forge.pipeline.adapter.spi.AdapterRegistry;
import com.shulex.forge.pipeline.adapter.spi.ContainerOrchestrationAdapter;
import com.shulex.forge.pipeline.infrastructure.entity.DeploymentRecordDO;
import com.shulex.forge.pipeline.infrastructure.mapper.DeploymentRecordMapper;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;

import java.util.Map;

import static org.assertj.core.api.Assertions.assertThat;
import static org.mockito.ArgumentMatchers.*;
import static org.mockito.Mockito.*;

class DeploymentServiceTest {

    private DeploymentService deploymentService;
    private DeploymentRecordMapper recordMapper;
    private AdapterRegistry adapterRegistry;
    private ContainerOrchestrationAdapter containerAdapter;

    @BeforeEach
    void setUp() {
        recordMapper = mock(DeploymentRecordMapper.class);
        adapterRegistry = mock(AdapterRegistry.class);
        containerAdapter = mock(ContainerOrchestrationAdapter.class);
        when(adapterRegistry.getContainerAdapter("ack")).thenReturn(containerAdapter);
        deploymentService = new DeploymentService(recordMapper, adapterRegistry);
    }

    @Test
    void deploy_createsDeploymentAndRecord() {
        when(recordMapper.insert(any())).thenReturn(1);
        when(recordMapper.updateById(any())).thenReturn(1);

        DeploymentRecordDO record = deploymentService.deploy(
                1L, "forge-dev", "forge-engine",
                "registry.cn-hangzhou.aliyuncs.com/forge/forge-engine:1",
                "repo-123", "main", 1, null);

        // mock adapter succeeds, so final status is RUNNING
        assertThat(record.getStatus()).isEqualTo("RUNNING");
        verify(containerAdapter).createOrUpdateDeployment(
                eq("forge-dev"), eq("forge-engine"),
                eq("registry.cn-hangzhou.aliyuncs.com/forge/forge-engine:1"),
                eq(1), any());
    }

    @Test
    void deploy_createsServiceForDeployment() {
        when(recordMapper.insert(any())).thenReturn(1);
        when(recordMapper.updateById(any())).thenReturn(1);

        deploymentService.deploy(1L, "forge-dev", "forge-engine",
                "img:1", "repo", "main", 1, null);

        verify(containerAdapter).createOrUpdateService(
                eq("forge-dev"), eq("forge-engine"),
                eq("ClusterIP"), any(), eq(8080), eq(8080));
    }
}
