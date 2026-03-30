package com.shulex.forge.engine.orchestration.service;

import com.fasterxml.jackson.databind.ObjectMapper;
import com.shulex.forge.engine.infrastructure.config.KafkaConfig;
import com.shulex.forge.engine.infrastructure.entity.TaskDO;
import com.shulex.forge.engine.infrastructure.entity.TaskStepDO;
import com.shulex.forge.engine.infrastructure.mapper.TaskStepMapper;
import com.shulex.forge.engine.execution.model.StepRequest;
import com.shulex.forge.engine.orchestration.model.StepStatus;
import com.shulex.forge.engine.orchestration.model.StepType;
import com.shulex.forge.engine.orchestration.model.TaskStatus;
import lombok.extern.slf4j.Slf4j;
import org.springframework.kafka.core.KafkaTemplate;
import org.springframework.stereotype.Service;

import java.util.List;

@Slf4j
@Service
public class TaskDispatcher {

    private final TaskService taskService;
    private final TaskStepMapper taskStepMapper;
    private final KafkaTemplate<String, String> kafkaTemplate;
    private final ObjectMapper objectMapper;

    public TaskDispatcher(TaskService taskService, TaskStepMapper taskStepMapper,
                          KafkaTemplate<String, String> kafkaTemplate) {
        this.taskService = taskService;
        this.taskStepMapper = taskStepMapper;
        this.kafkaTemplate = kafkaTemplate;
        this.objectMapper = new ObjectMapper();
    }

    public void startTask(Long taskId) {
        TaskDO task = taskService.getTask(taskId);
        createSteps(task);
        taskService.transitionStatus(taskId, TaskStatus.ANALYZING);
        dispatchNextStep(taskId);
    }

    public void dispatchNextStep(Long taskId) {
        List<TaskStepDO> steps = taskService.getSteps(taskId);
        TaskDO task = taskService.getTask(taskId);

        for (TaskStepDO step : steps) {
            if (StepStatus.PENDING.name().equals(step.getStatus())) {
                step.setStatus(StepStatus.RUNNING.name());
                taskStepMapper.updateById(step);

                updateTaskStatusForStep(taskId, step.getStepType());

                StepRequest request = new StepRequest();
                request.setTaskId(taskId);
                request.setStepId(step.getId());
                request.setStepType(step.getStepType());
                request.setAdapterType("codeup");
                request.setRepoId(task.getRepoId());
                request.setBranchName(task.getBranchName());
                request.setRequirement(task.getRequirement());

                try {
                    String json = objectMapper.writeValueAsString(request);
                    kafkaTemplate.send(KafkaConfig.TOPIC_STEP_REQUEST, String.valueOf(taskId), json);
                    log.info("派发步骤: task={}, step={}, type={}", taskId, step.getId(), step.getStepType());
                } catch (Exception e) {
                    log.error("派发步骤失败", e);
                }
                return;
            }
        }
        log.info("所有步骤已完成: task={}", taskId);
    }

    private void createSteps(TaskDO task) {
        StepType[] stepSequence = {
                StepType.ANALYZE, StepType.PLAN, StepType.RISK_ASSESS_INIT,
                StepType.GENERATE_CONTRACT, StepType.GENERATE_CODE,
                StepType.REVIEW, StepType.RISK_ASSESS_FINAL,
                StepType.COMMIT, StepType.CREATE_MR
        };
        for (int i = 0; i < stepSequence.length; i++) {
            TaskStepDO step = new TaskStepDO();
            step.setTaskId(task.getId());
            step.setStepType(stepSequence[i].name());
            step.setStepOrder(i + 1);
            step.setStatus(StepStatus.PENDING.name());
            step.setInputTokens(0L);
            step.setOutputTokens(0L);
            step.setRetryCount(0);
            taskStepMapper.insert(step);
        }
    }

    private void updateTaskStatusForStep(Long taskId, String stepType) {
        try {
            StepType st = StepType.valueOf(stepType);
            TaskStatus targetStatus = switch (st) {
                case ANALYZE -> TaskStatus.ANALYZING;
                case PLAN, RISK_ASSESS_INIT -> TaskStatus.PLANNING;
                case GENERATE_CONTRACT, GENERATE_CODE -> TaskStatus.GENERATING;
                case REVIEW, RISK_ASSESS_FINAL -> TaskStatus.REVIEWING;
                case COMMIT, CREATE_MR -> TaskStatus.DEPLOYING;
                default -> null;
            };
            if (targetStatus != null) {
                TaskDO task = taskService.getTask(taskId);
                TaskStatus current = TaskStatus.valueOf(task.getStatus());
                if (current != targetStatus && com.shulex.forge.engine.orchestration.statemachine.TaskStateMachine.transition(current, targetStatus)) {
                    taskService.transitionStatus(taskId, targetStatus);
                }
            }
        } catch (Exception e) {
            log.debug("状态更新跳过: {}", e.getMessage());
        }
    }
}
