package com.shulex.forge.pipeline.devops.service;

import com.shulex.forge.pipeline.adapter.spi.AdapterRegistry;
import com.shulex.forge.pipeline.adapter.spi.ContainerOrchestrationAdapter;
import com.shulex.forge.pipeline.infrastructure.entity.DeploymentRecordDO;
import com.shulex.forge.pipeline.infrastructure.mapper.DeploymentRecordMapper;
import lombok.extern.slf4j.Slf4j;
import org.springframework.stereotype.Service;

import java.util.Map;

@Slf4j
@Service
public class DeploymentService {

    private final DeploymentRecordMapper recordMapper;
    private final AdapterRegistry adapterRegistry;

    public DeploymentService(DeploymentRecordMapper recordMapper, AdapterRegistry adapterRegistry) {
        this.recordMapper = recordMapper;
        this.adapterRegistry = adapterRegistry;
    }

    public DeploymentRecordDO deploy(Long tenantId, String namespace, String deploymentName,
                                      String image, String repoId, String branch,
                                      int replicas, Long pipelineExecutionId) {
        DeploymentRecordDO record = new DeploymentRecordDO();
        record.setTenantId(tenantId);
        record.setNamespace(namespace);
        record.setDeploymentName(deploymentName);
        record.setImage(image);
        record.setRepoId(repoId);
        record.setBranch(branch);
        record.setReplicas(replicas);
        record.setStatus("DEPLOYING");
        record.setPipelineExecutionId(pipelineExecutionId);
        recordMapper.insert(record);

        try {
            ContainerOrchestrationAdapter adapter = adapterRegistry.getContainerAdapter("ack");
            adapter.createOrUpdateDeployment(namespace, deploymentName, image, replicas,
                    Map.of("APP_NAME", deploymentName));
            adapter.createOrUpdateService(namespace, deploymentName, "ClusterIP",
                    Map.of("app", deploymentName), 8080, 8080);
            record.setStatus("RUNNING");
            log.info("部署成功: namespace={}, name={}", namespace, deploymentName);
        } catch (Exception e) {
            record.setStatus("FAILED");
            record.setErrorMessage(e.getMessage());
            log.error("部署失败: namespace={}, name={}", namespace, deploymentName, e);
        }

        recordMapper.updateById(record);
        return record;
    }
}
