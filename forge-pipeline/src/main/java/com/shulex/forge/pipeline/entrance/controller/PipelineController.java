package com.shulex.forge.pipeline.entrance.controller;

import com.shulex.forge.pipeline.common.Result;
import com.shulex.forge.pipeline.devops.service.WebhookDispatcher;
import com.shulex.forge.pipeline.entrance.vo.PipelineExecutionVO;
import com.shulex.forge.pipeline.entrance.vo.TriggerPipelineRequest;
import com.shulex.forge.pipeline.infrastructure.entity.PipelineExecutionDO;
import com.shulex.forge.pipeline.infrastructure.mapper.PipelineExecutionMapper;
import jakarta.validation.Valid;
import org.springframework.web.bind.annotation.*;

@RestController
@RequestMapping("/api/pipelines")
public class PipelineController {

    private final WebhookDispatcher webhookDispatcher;
    private final PipelineExecutionMapper executionMapper;

    public PipelineController(WebhookDispatcher webhookDispatcher, PipelineExecutionMapper executionMapper) {
        this.webhookDispatcher = webhookDispatcher;
        this.executionMapper = executionMapper;
    }

    @PostMapping("/trigger")
    public Result<Void> trigger(@Valid @RequestBody TriggerPipelineRequest request) {
        webhookDispatcher.onPush(request.getTenantId(), request.getRepoId(),
                request.getBranch(), "MANUAL");
        return Result.ok(null);
    }

    @GetMapping("/{id}")
    public Result<PipelineExecutionVO> get(@PathVariable("id") Long id) {
        PipelineExecutionDO exec = executionMapper.selectById(id);
        if (exec == null) return Result.fail(40400, "Pipeline execution not found");
        return Result.ok(toVO(exec));
    }

    private PipelineExecutionVO toVO(PipelineExecutionDO exec) {
        return PipelineExecutionVO.builder()
                .id(exec.getId())
                .repoId(exec.getRepoId())
                .branch(exec.getBranch())
                .projectType(exec.getProjectType())
                .status(exec.getStatus())
                .compilePassed(exec.getCompilePassed() != null ? exec.getCompilePassed() == 1 : null)
                .testPassed(exec.getTestPassed() != null ? exec.getTestPassed() == 1 : null)
                .reviewPassed(exec.getReviewPassed() != null ? exec.getReviewPassed() == 1 : null)
                .qualityGatePassed(exec.getQualityGatePassed() != null ? exec.getQualityGatePassed() == 1 : null)
                .triggerType(exec.getTriggerType())
                .gmtCreate(exec.getGmtCreate())
                .build();
    }
}
