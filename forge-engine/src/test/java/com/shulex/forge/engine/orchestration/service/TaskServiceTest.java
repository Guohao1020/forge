package com.shulex.forge.engine.orchestration.service;

import com.shulex.forge.engine.common.BizException;
import com.shulex.forge.engine.infrastructure.entity.TaskDO;
import com.shulex.forge.engine.infrastructure.entity.TaskStepDO;
import com.shulex.forge.engine.infrastructure.mapper.TaskMapper;
import com.shulex.forge.engine.infrastructure.mapper.TaskStepMapper;
import com.shulex.forge.engine.orchestration.model.TaskStatus;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;

import static org.assertj.core.api.Assertions.*;
import static org.mockito.ArgumentMatchers.any;
import static org.mockito.Mockito.*;

class TaskServiceTest {

    private TaskService taskService;
    private TaskMapper taskMapper;
    private TaskStepMapper taskStepMapper;
    private KillSwitchService killSwitchService;
    private RiskAssessor riskAssessor;

    @BeforeEach
    void setUp() {
        taskMapper = mock(TaskMapper.class);
        taskStepMapper = mock(TaskStepMapper.class);
        killSwitchService = mock(KillSwitchService.class);
        riskAssessor = mock(RiskAssessor.class);
        taskService = new TaskService(taskMapper, taskStepMapper, killSwitchService, riskAssessor);
    }

    @Test
    void createTask_insertsAndReturns() {
        when(killSwitchService.isNewTaskAllowed()).thenReturn(true);
        when(taskMapper.insert(any())).thenReturn(1);

        TaskDO task = taskService.createTask(1L, 1L, "创建用户服务", "GENERATE", "repo-123");
        assertThat(task.getStatus()).isEqualTo(TaskStatus.SUBMITTED.name());
        verify(taskMapper).insert(any());
    }

    @Test
    void createTask_throwsWhenKillSwitchActive() {
        when(killSwitchService.isNewTaskAllowed()).thenReturn(false);

        assertThatThrownBy(() -> taskService.createTask(1L, 1L, "test", "GENERATE", "repo"))
                .isInstanceOf(BizException.class);
    }

    @Test
    void transitionStatus_updatesOnValid() {
        TaskDO task = new TaskDO();
        task.setId(1L);
        task.setStatus(TaskStatus.SUBMITTED.name());
        when(taskMapper.selectById(1L)).thenReturn(task);
        when(taskMapper.updateById(any())).thenReturn(1);

        taskService.transitionStatus(1L, TaskStatus.ANALYZING);
        assertThat(task.getStatus()).isEqualTo(TaskStatus.ANALYZING.name());
    }

    @Test
    void transitionStatus_throwsOnInvalid() {
        TaskDO task = new TaskDO();
        task.setId(1L);
        task.setStatus(TaskStatus.DONE.name());
        when(taskMapper.selectById(1L)).thenReturn(task);

        assertThatThrownBy(() -> taskService.transitionStatus(1L, TaskStatus.SUBMITTED))
                .isInstanceOf(BizException.class);
    }
}
