package com.shulex.forge.pipeline.devops.service;

import com.baomidou.mybatisplus.core.conditions.query.LambdaQueryWrapper;
import com.shulex.forge.pipeline.adapter.spi.AdapterRegistry;
import com.shulex.forge.pipeline.adapter.spi.ContainerOrchestrationAdapter;
import com.shulex.forge.pipeline.devops.model.EnvironmentType;
import com.shulex.forge.pipeline.infrastructure.entity.EnvironmentDO;
import com.shulex.forge.pipeline.infrastructure.mapper.EnvironmentMapper;
import lombok.extern.slf4j.Slf4j;
import org.springframework.stereotype.Service;

import java.time.LocalDateTime;
import java.util.List;
import java.util.Map;

@Slf4j
@Service
public class EnvironmentService {

    private final EnvironmentMapper environmentMapper;
    private final AdapterRegistry adapterRegistry;

    public EnvironmentService(EnvironmentMapper environmentMapper, AdapterRegistry adapterRegistry) {
        this.environmentMapper = environmentMapper;
        this.adapterRegistry = adapterRegistry;
    }

    public EnvironmentDO createTemporaryEnvironment(Long tenantId, String repoId, String branch, Long taskId) {
        String namespace = "temp-" + taskId + "-" + System.currentTimeMillis() % 10000;

        EnvironmentDO env = new EnvironmentDO();
        env.setTenantId(tenantId);
        env.setName("temp-task-" + taskId);
        env.setEnvType(EnvironmentType.TEMPORARY.name());
        env.setNamespace(namespace);
        env.setBoundBranch(branch);
        env.setStatus("ACTIVE");
        env.setAutoDestroyAt(LocalDateTime.now().plusMinutes(30));
        env.setRepoId(repoId);
        env.setTaskId(taskId);
        environmentMapper.insert(env);

        try {
            ContainerOrchestrationAdapter adapter = adapterRegistry.getContainerAdapter("ack");
            adapter.createNamespace(namespace, Map.of(
                    "forge.io/env-type", "temporary",
                    "forge.io/task-id", String.valueOf(taskId)
            ));
            log.info("临时环境创建: namespace={}, task={}", namespace, taskId);
        } catch (Exception e) {
            log.error("创建临时环境失败: namespace={}", namespace, e);
            env.setStatus("FAILED");
            environmentMapper.updateById(env);
        }

        return env;
    }

    public void destroyEnvironment(Long environmentId) {
        EnvironmentDO env = environmentMapper.selectById(environmentId);
        if (env == null) return;

        try {
            ContainerOrchestrationAdapter adapter = adapterRegistry.getContainerAdapter("ack");
            adapter.deleteNamespace(env.getNamespace());
            env.setStatus("DESTROYED");
            log.info("环境已销毁: namespace={}", env.getNamespace());
        } catch (Exception e) {
            env.setStatus("DESTROYING");
            log.error("销毁环境失败: namespace={}", env.getNamespace(), e);
        }
        environmentMapper.updateById(env);
    }

    public EnvironmentDO findFixedEnvironmentByBranch(Long tenantId, String branch) {
        return environmentMapper.selectOne(new LambdaQueryWrapper<EnvironmentDO>()
                .eq(EnvironmentDO::getTenantId, tenantId)
                .eq(EnvironmentDO::getEnvType, EnvironmentType.FIXED.name())
                .eq(EnvironmentDO::getBoundBranch, branch)
                .eq(EnvironmentDO::getStatus, "ACTIVE"));
    }

    public List<EnvironmentDO> listEnvironments(Long tenantId) {
        return environmentMapper.selectList(new LambdaQueryWrapper<EnvironmentDO>()
                .eq(EnvironmentDO::getTenantId, tenantId)
                .orderByDesc(EnvironmentDO::getGmtCreate));
    }

    public List<EnvironmentDO> findExpiredTemporaryEnvironments() {
        return environmentMapper.selectList(new LambdaQueryWrapper<EnvironmentDO>()
                .eq(EnvironmentDO::getEnvType, EnvironmentType.TEMPORARY.name())
                .eq(EnvironmentDO::getStatus, "ACTIVE")
                .le(EnvironmentDO::getAutoDestroyAt, LocalDateTime.now()));
    }
}
