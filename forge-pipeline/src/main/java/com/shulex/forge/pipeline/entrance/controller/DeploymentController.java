package com.shulex.forge.pipeline.entrance.controller;

import com.shulex.forge.pipeline.common.Result;
import com.shulex.forge.pipeline.devops.service.DeploymentService;
import com.shulex.forge.pipeline.entrance.vo.DeployRequest;
import com.shulex.forge.pipeline.entrance.vo.DeploymentRecordVO;
import com.shulex.forge.pipeline.infrastructure.entity.DeploymentRecordDO;
import jakarta.validation.Valid;
import org.springframework.web.bind.annotation.*;

@RestController
@RequestMapping("/api/deployments")
public class DeploymentController {

    private final DeploymentService deploymentService;

    public DeploymentController(DeploymentService deploymentService) {
        this.deploymentService = deploymentService;
    }

    @PostMapping
    public Result<DeploymentRecordVO> deploy(@Valid @RequestBody DeployRequest request) {
        DeploymentRecordDO record = deploymentService.deploy(
                request.getTenantId(), request.getNamespace(), request.getDeploymentName(),
                request.getImage(), request.getRepoId(), request.getBranch(),
                request.getReplicas(), null);
        return Result.ok(toVO(record));
    }

    private DeploymentRecordVO toVO(DeploymentRecordDO record) {
        return DeploymentRecordVO.builder()
                .id(record.getId())
                .namespace(record.getNamespace())
                .deploymentName(record.getDeploymentName())
                .image(record.getImage())
                .status(record.getStatus())
                .branch(record.getBranch())
                .gmtCreate(record.getGmtCreate())
                .build();
    }
}
