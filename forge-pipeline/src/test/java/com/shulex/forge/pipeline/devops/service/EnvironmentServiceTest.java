package com.shulex.forge.pipeline.devops.service;

import com.shulex.forge.pipeline.adapter.spi.AdapterRegistry;
import com.shulex.forge.pipeline.adapter.spi.ContainerOrchestrationAdapter;
import com.shulex.forge.pipeline.devops.model.EnvironmentType;
import com.shulex.forge.pipeline.infrastructure.entity.EnvironmentDO;
import com.shulex.forge.pipeline.infrastructure.mapper.EnvironmentMapper;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;

import java.util.Map;

import static org.assertj.core.api.Assertions.assertThat;
import static org.mockito.ArgumentMatchers.*;
import static org.mockito.Mockito.*;

class EnvironmentServiceTest {

    private EnvironmentService environmentService;
    private EnvironmentMapper environmentMapper;
    private AdapterRegistry adapterRegistry;
    private ContainerOrchestrationAdapter containerAdapter;

    @BeforeEach
    void setUp() {
        environmentMapper = mock(EnvironmentMapper.class);
        adapterRegistry = mock(AdapterRegistry.class);
        containerAdapter = mock(ContainerOrchestrationAdapter.class);
        when(adapterRegistry.getContainerAdapter("ack")).thenReturn(containerAdapter);
        environmentService = new EnvironmentService(environmentMapper, adapterRegistry);
    }

    @Test
    void createTemporaryEnvironment_createsNamespace() {
        when(environmentMapper.insert(any())).thenReturn(1);

        EnvironmentDO env = environmentService.createTemporaryEnvironment(
                1L, "repo-123", "ai/feature-login", 100L);

        assertThat(env.getEnvType()).isEqualTo(EnvironmentType.TEMPORARY.name());
        assertThat(env.getNamespace()).startsWith("temp-");
        assertThat(env.getAutoDestroyAt()).isNotNull();
        verify(containerAdapter).createNamespace(eq(env.getNamespace()), any());
    }

    @Test
    void destroyEnvironment_deletesNamespace() {
        EnvironmentDO env = new EnvironmentDO();
        env.setId(1L);
        env.setNamespace("temp-123");
        env.setStatus("ACTIVE");
        when(environmentMapper.selectById(1L)).thenReturn(env);
        when(environmentMapper.updateById(any())).thenReturn(1);

        environmentService.destroyEnvironment(1L);

        verify(containerAdapter).deleteNamespace("temp-123");
        assertThat(env.getStatus()).isEqualTo("DESTROYED");
    }

    @Test
    void findFixedEnvironmentByBranch_returnsDev() {
        EnvironmentDO dev = new EnvironmentDO();
        dev.setName("dev");
        dev.setBoundBranch("develop");
        when(environmentMapper.selectOne(any())).thenReturn(dev);

        EnvironmentDO result = environmentService.findFixedEnvironmentByBranch(1L, "develop");
        assertThat(result.getName()).isEqualTo("dev");
    }
}
