package com.shulex.forge.pipeline.devops.service;

import com.shulex.forge.pipeline.devops.model.ProjectType;
import com.shulex.forge.pipeline.infrastructure.entity.PipelineExecutionDO;
import com.shulex.forge.pipeline.infrastructure.mapper.PipelineExecutionMapper;
import lombok.extern.slf4j.Slf4j;
import org.springframework.stereotype.Service;

@Slf4j
@Service
public class WebhookDispatcher {

    private final PipelineExecutionMapper executionMapper;
    private final PipelineTemplateService templateService;
    private final EnvironmentService environmentService;
    private final DeploymentService deploymentService;
    private final QualityGateService qualityGateService;

    public WebhookDispatcher(PipelineExecutionMapper executionMapper,
                             PipelineTemplateService templateService,
                             EnvironmentService environmentService,
                             DeploymentService deploymentService,
                             QualityGateService qualityGateService) {
        this.executionMapper = executionMapper;
        this.templateService = templateService;
        this.environmentService = environmentService;
        this.deploymentService = deploymentService;
        this.qualityGateService = qualityGateService;
    }

    public void onPush(Long tenantId, String repoId, String branch, String triggerType) {
        log.info("收到推送事件: tenant={}, repo={}, branch={}", tenantId, repoId, branch);

        PipelineExecutionDO execution = new PipelineExecutionDO();
        execution.setTenantId(tenantId);
        execution.setRepoId(repoId);
        execution.setBranch(branch);
        execution.setProjectType(ProjectType.JAVA_SERVICE.name());
        execution.setStatus("PENDING");
        execution.setTriggerType(triggerType);
        executionMapper.insert(execution);

        // AI 分支 → 创建临时环境
        if (branch.startsWith("ai/")) {
            environmentService.createTemporaryEnvironment(tenantId, repoId, branch, null);
        }

        log.info("流水线执行已创建: id={}", execution.getId());
    }

    public void onMergeRequestMerged(Long tenantId, String repoId, String sourceBranch, String targetBranch) {
        log.info("MR 已合并: source={} -> target={}", sourceBranch, targetBranch);
        // 触发目标分支对应环境的部署
        var env = environmentService.findFixedEnvironmentByBranch(tenantId, targetBranch);
        if (env != null) {
            log.info("触发固定环境部署: env={}, branch={}", env.getName(), targetBranch);
        }
    }
}
