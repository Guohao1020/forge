package com.shulex.forge.pipeline.entrance.controller;

import com.shulex.forge.pipeline.common.Result;
import com.shulex.forge.pipeline.devops.service.EnvironmentService;
import com.shulex.forge.pipeline.entrance.vo.CreateEnvironmentRequest;
import com.shulex.forge.pipeline.entrance.vo.EnvironmentVO;
import com.shulex.forge.pipeline.infrastructure.entity.EnvironmentDO;
import jakarta.validation.Valid;
import org.springframework.web.bind.annotation.*;

import java.util.List;

@RestController
@RequestMapping("/api/environments")
public class EnvironmentController {

    private final EnvironmentService environmentService;

    public EnvironmentController(EnvironmentService environmentService) {
        this.environmentService = environmentService;
    }

    @PostMapping("/temporary")
    public Result<EnvironmentVO> createTemporary(@Valid @RequestBody CreateEnvironmentRequest request) {
        EnvironmentDO env = environmentService.createTemporaryEnvironment(
                request.getTenantId(), request.getRepoId(), request.getBranch(), request.getTaskId());
        return Result.ok(toVO(env));
    }

    @DeleteMapping("/{id}")
    public Result<Void> destroy(@PathVariable("id") Long id) {
        environmentService.destroyEnvironment(id);
        return Result.ok(null);
    }

    @GetMapping
    public Result<List<EnvironmentVO>> list(@RequestParam("tenantId") Long tenantId) {
        return Result.ok(environmentService.listEnvironments(tenantId).stream()
                .map(this::toVO).toList());
    }

    private EnvironmentVO toVO(EnvironmentDO env) {
        return EnvironmentVO.builder()
                .id(env.getId())
                .name(env.getName())
                .envType(env.getEnvType())
                .namespace(env.getNamespace())
                .boundBranch(env.getBoundBranch())
                .status(env.getStatus())
                .autoDestroyAt(env.getAutoDestroyAt())
                .gmtCreate(env.getGmtCreate())
                .build();
    }
}
