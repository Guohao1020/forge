package com.shulex.forge.engine.orchestration.listener;

import com.fasterxml.jackson.databind.ObjectMapper;
import com.shulex.forge.engine.execution.model.StepResult;
import com.shulex.forge.engine.infrastructure.config.KafkaConfig;
import com.shulex.forge.engine.infrastructure.entity.TaskStepDO;
import com.shulex.forge.engine.infrastructure.mapper.TaskStepMapper;
import com.shulex.forge.engine.orchestration.model.StepStatus;
import com.shulex.forge.engine.orchestration.model.TaskStatus;
import com.shulex.forge.engine.orchestration.service.TaskDispatcher;
import com.shulex.forge.engine.orchestration.service.TaskService;
import com.shulex.forge.engine.orchestration.service.TokenUsageService;
import lombok.extern.slf4j.Slf4j;
import org.springframework.kafka.annotation.KafkaListener;
import org.springframework.stereotype.Component;

@Slf4j
@Component
public class StepResultListener {

    private final TaskService taskService;
    private final TaskDispatcher taskDispatcher;
    private final TaskStepMapper taskStepMapper;
    private final TokenUsageService tokenUsageService;
    private final ObjectMapper objectMapper = new ObjectMapper();

    public StepResultListener(TaskService taskService, TaskDispatcher taskDispatcher,
                              TaskStepMapper taskStepMapper, TokenUsageService tokenUsageService) {
        this.taskService = taskService;
        this.taskDispatcher = taskDispatcher;
        this.taskStepMapper = taskStepMapper;
        this.tokenUsageService = tokenUsageService;
    }

    @KafkaListener(topics = KafkaConfig.TOPIC_STEP_RESULT, groupId = "forge-engine-orchestrator")
    public void onStepResult(String message) {
        try {
            StepResult result = objectMapper.readValue(message, StepResult.class);
            log.info("收到步骤结果: task={}, step={}, success={}",
                    result.getTaskId(), result.getStepId(), result.isSuccess());

            TaskStepDO step = taskStepMapper.selectById(result.getStepId());
            if (step == null) return;

            step.setStatus(result.isSuccess() ? StepStatus.SUCCESS.name() : StepStatus.FAILED.name());
            step.setOutputSnapshot(result.getOutputData());
            step.setInputTokens(result.getInputTokens());
            step.setOutputTokens(result.getOutputTokens());
            step.setErrorMessage(result.getErrorMessage());
            taskStepMapper.updateById(step);

            if (result.getInputTokens() > 0 || result.getOutputTokens() > 0) {
                tokenUsageService.recordCall(result.getTaskId(), result.getStepId(),
                        "claude-sonnet-4-20250514", result.getStepType(),
                        result.getInputTokens(), result.getOutputTokens(), 0);
                taskService.updateTokenUsage(result.getTaskId(),
                        result.getInputTokens(), result.getOutputTokens());
            }

            if (result.isSuccess()) {
                taskDispatcher.dispatchNextStep(result.getTaskId());
            } else {
                if (step.getRetryCount() < 3) {
                    step.setRetryCount(step.getRetryCount() + 1);
                    step.setStatus(StepStatus.PENDING.name());
                    taskStepMapper.updateById(step);
                    taskDispatcher.dispatchNextStep(result.getTaskId());
                } else {
                    taskService.transitionStatus(result.getTaskId(), TaskStatus.FAILED);
                }
            }
        } catch (Exception e) {
            log.error("处理步骤结果失败", e);
        }
    }
}
