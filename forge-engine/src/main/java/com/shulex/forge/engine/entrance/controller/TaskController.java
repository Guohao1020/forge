package com.shulex.forge.engine.entrance.controller;

import com.shulex.forge.engine.common.Result;
import com.shulex.forge.engine.entrance.vo.CreateTaskRequest;
import com.shulex.forge.engine.entrance.vo.TaskStepVO;
import com.shulex.forge.engine.entrance.vo.TaskVO;
import com.shulex.forge.engine.infrastructure.entity.TaskDO;
import com.shulex.forge.engine.orchestration.service.TaskDispatcher;
import com.shulex.forge.engine.orchestration.service.TaskService;
import jakarta.validation.Valid;
import org.springframework.web.bind.annotation.*;

import java.util.List;

@RestController
@RequestMapping("/api/tasks")
public class TaskController {

    private final TaskService taskService;
    private final TaskDispatcher taskDispatcher;

    public TaskController(TaskService taskService, TaskDispatcher taskDispatcher) {
        this.taskService = taskService;
        this.taskDispatcher = taskDispatcher;
    }

    @PostMapping
    public Result<TaskVO> createTask(@Valid @RequestBody CreateTaskRequest request) {
        TaskDO task = taskService.createTask(
                request.getTenantId(), request.getUserId(),
                request.getRequirement(), request.getTaskType(), request.getRepoId());
        taskDispatcher.startTask(task.getId());
        return Result.ok(toVO(task));
    }

    @GetMapping("/{taskId}")
    public Result<TaskVO> getTask(@PathVariable("taskId") Long taskId) {
        return Result.ok(toVO(taskService.getTask(taskId)));
    }

    @GetMapping
    public Result<List<TaskVO>> listTasks(
            @RequestParam("tenantId") Long tenantId,
            @RequestParam("userId") Long userId) {
        return Result.ok(taskService.listTasks(tenantId, userId).stream()
                .map(this::toVO).toList());
    }

    @GetMapping("/{taskId}/steps")
    public Result<List<TaskStepVO>> getSteps(@PathVariable("taskId") Long taskId) {
        return Result.ok(taskService.getSteps(taskId).stream()
                .map(s -> TaskStepVO.builder()
                        .id(s.getId())
                        .stepType(s.getStepType())
                        .stepOrder(s.getStepOrder())
                        .status(s.getStatus())
                        .inputTokens(s.getInputTokens())
                        .outputTokens(s.getOutputTokens())
                        .retryCount(s.getRetryCount())
                        .errorMessage(s.getErrorMessage())
                        .build())
                .toList());
    }

    @PostMapping("/{taskId}/cancel")
    public Result<Void> cancelTask(@PathVariable("taskId") Long taskId) {
        taskService.transitionStatus(taskId, com.shulex.forge.engine.orchestration.model.TaskStatus.CANCELLED);
        return Result.ok(null);
    }

    private TaskVO toVO(TaskDO task) {
        return TaskVO.builder()
                .id(task.getId())
                .tenantId(task.getTenantId())
                .userId(task.getUserId())
                .requirement(task.getRequirement())
                .taskType(task.getTaskType())
                .status(task.getStatus())
                .riskLevel(task.getRiskLevel())
                .repoId(task.getRepoId())
                .branchName(task.getBranchName())
                .mrId(task.getMrId())
                .reviewScore(task.getReviewScore())
                .totalInputTokens(task.getTotalInputTokens())
                .totalOutputTokens(task.getTotalOutputTokens())
                .gmtCreate(task.getGmtCreate())
                .build();
    }
}
