package com.shulex.forge.engine.entrance.controller;

import com.shulex.forge.engine.common.Result;
import com.shulex.forge.engine.entrance.vo.TokenUsageVO;
import com.shulex.forge.engine.infrastructure.entity.TaskDO;
import com.shulex.forge.engine.orchestration.service.TaskService;
import org.springframework.web.bind.annotation.*;

@RestController
@RequestMapping("/api/token-usage")
public class TokenUsageController {

    private final TaskService taskService;

    public TokenUsageController(TaskService taskService) {
        this.taskService = taskService;
    }

    @GetMapping("/{taskId}")
    public Result<TokenUsageVO> getUsage(@PathVariable("taskId") Long taskId) {
        TaskDO task = taskService.getTask(taskId);
        return Result.ok(TokenUsageVO.builder()
                .taskId(task.getId())
                .totalInputTokens(task.getTotalInputTokens())
                .totalOutputTokens(task.getTotalOutputTokens())
                .build());
    }
}
