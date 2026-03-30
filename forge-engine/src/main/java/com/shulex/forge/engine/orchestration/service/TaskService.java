package com.shulex.forge.engine.orchestration.service;

import com.baomidou.mybatisplus.core.conditions.query.LambdaQueryWrapper;
import com.shulex.forge.engine.common.BizException;
import com.shulex.forge.engine.common.ErrorCode;
import com.shulex.forge.engine.infrastructure.entity.TaskDO;
import com.shulex.forge.engine.infrastructure.entity.TaskStepDO;
import com.shulex.forge.engine.infrastructure.mapper.TaskMapper;
import com.shulex.forge.engine.infrastructure.mapper.TaskStepMapper;
import com.shulex.forge.engine.orchestration.model.TaskStatus;
import com.shulex.forge.engine.orchestration.statemachine.TaskStateMachine;
import lombok.extern.slf4j.Slf4j;
import org.springframework.stereotype.Service;

import java.util.List;

@Slf4j
@Service
public class TaskService {

    private final TaskMapper taskMapper;
    private final TaskStepMapper taskStepMapper;
    private final KillSwitchService killSwitchService;
    private final RiskAssessor riskAssessor;

    public TaskService(TaskMapper taskMapper, TaskStepMapper taskStepMapper,
                       KillSwitchService killSwitchService, RiskAssessor riskAssessor) {
        this.taskMapper = taskMapper;
        this.taskStepMapper = taskStepMapper;
        this.killSwitchService = killSwitchService;
        this.riskAssessor = riskAssessor;
    }

    public TaskDO createTask(Long tenantId, Long userId, String requirement, String taskType, String repoId) {
        if (!killSwitchService.isNewTaskAllowed()) {
            throw new BizException(ErrorCode.KILL_SWITCH_ACTIVE);
        }
        TaskDO task = new TaskDO();
        task.setTenantId(tenantId);
        task.setUserId(userId);
        task.setRequirement(requirement);
        task.setTaskType(taskType);
        task.setStatus(TaskStatus.SUBMITTED.name());
        task.setRepoId(repoId);
        task.setTotalInputTokens(0L);
        task.setTotalOutputTokens(0L);
        taskMapper.insert(task);
        log.info("创建任务: id={}, tenant={}, user={}", task.getId(), tenantId, userId);
        return task;
    }

    public TaskDO getTask(Long taskId) {
        TaskDO task = taskMapper.selectById(taskId);
        if (task == null) {
            throw new BizException(ErrorCode.TASK_NOT_FOUND);
        }
        return task;
    }

    public List<TaskDO> listTasks(Long tenantId, Long userId) {
        return taskMapper.selectList(new LambdaQueryWrapper<TaskDO>()
                .eq(TaskDO::getTenantId, tenantId)
                .eq(TaskDO::getUserId, userId)
                .orderByDesc(TaskDO::getGmtCreate));
    }

    public void transitionStatus(Long taskId, TaskStatus newStatus) {
        TaskDO task = getTask(taskId);
        TaskStatus currentStatus = TaskStatus.valueOf(task.getStatus());
        if (!TaskStateMachine.transition(currentStatus, newStatus)) {
            throw new BizException(ErrorCode.TASK_INVALID_STATUS,
                    "不允许从 " + currentStatus + " 转换到 " + newStatus);
        }
        task.setStatus(newStatus.name());
        taskMapper.updateById(task);
        log.info("任务状态变更: id={}, {} -> {}", taskId, currentStatus, newStatus);
    }

    public void updateTokenUsage(Long taskId, long inputTokens, long outputTokens) {
        TaskDO task = getTask(taskId);
        task.setTotalInputTokens(task.getTotalInputTokens() + inputTokens);
        task.setTotalOutputTokens(task.getTotalOutputTokens() + outputTokens);
        taskMapper.updateById(task);
    }

    public List<TaskStepDO> getSteps(Long taskId) {
        return taskStepMapper.selectList(new LambdaQueryWrapper<TaskStepDO>()
                .eq(TaskStepDO::getTaskId, taskId)
                .orderByAsc(TaskStepDO::getStepOrder));
    }
}
