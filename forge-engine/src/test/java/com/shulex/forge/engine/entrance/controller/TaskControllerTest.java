package com.shulex.forge.engine.entrance.controller;

import com.fasterxml.jackson.databind.ObjectMapper;
import com.shulex.forge.engine.entrance.vo.CreateTaskRequest;
import com.shulex.forge.engine.infrastructure.entity.TaskDO;
import com.shulex.forge.engine.orchestration.service.TaskDispatcher;
import com.shulex.forge.engine.orchestration.service.TaskService;
import org.junit.jupiter.api.Test;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.boot.test.autoconfigure.web.servlet.AutoConfigureMockMvc;
import org.springframework.boot.test.context.SpringBootTest;
import org.springframework.boot.test.mock.mockito.MockBean;
import org.springframework.data.redis.core.StringRedisTemplate;
import org.springframework.http.MediaType;
import org.springframework.kafka.core.KafkaTemplate;
import org.springframework.test.context.ActiveProfiles;
import org.springframework.test.web.servlet.MockMvc;

import static org.mockito.ArgumentMatchers.*;
import static org.mockito.Mockito.*;
import static org.springframework.test.web.servlet.request.MockMvcRequestBuilders.*;
import static org.springframework.test.web.servlet.result.MockMvcResultMatchers.*;

@SpringBootTest
@AutoConfigureMockMvc
@ActiveProfiles("test")
class TaskControllerTest {

    @Autowired
    private MockMvc mockMvc;
    @Autowired
    private ObjectMapper objectMapper;
    @MockBean
    private TaskService taskService;
    @MockBean
    private TaskDispatcher taskDispatcher;
    @MockBean
    private StringRedisTemplate redisTemplate;
    @MockBean
    private KafkaTemplate<String, String> kafkaTemplate;

    @Test
    void createTask_returns200() throws Exception {
        TaskDO task = new TaskDO();
        task.setId(1L);
        task.setTenantId(1L);
        task.setUserId(1L);
        task.setRequirement("创建用户服务");
        task.setTaskType("GENERATE");
        task.setStatus("SUBMITTED");
        task.setRepoId("repo-123");
        task.setTotalInputTokens(0L);
        task.setTotalOutputTokens(0L);

        when(taskService.createTask(eq(1L), eq(1L), eq("创建用户服务"), eq("GENERATE"), eq("repo-123")))
                .thenReturn(task);

        CreateTaskRequest request = new CreateTaskRequest();
        request.setTenantId(1L);
        request.setUserId(1L);
        request.setRequirement("创建用户服务");
        request.setRepoId("repo-123");

        mockMvc.perform(post("/api/tasks")
                        .contentType(MediaType.APPLICATION_JSON)
                        .content(objectMapper.writeValueAsString(request)))
                .andExpect(status().isOk())
                .andExpect(jsonPath("$.data.id").value(1))
                .andExpect(jsonPath("$.data.status").value("SUBMITTED"));

        verify(taskDispatcher).startTask(1L);
    }

    @Test
    void createTask_returns400OnMissingFields() throws Exception {
        mockMvc.perform(post("/api/tasks")
                        .contentType(MediaType.APPLICATION_JSON)
                        .content("{}"))
                .andExpect(status().isBadRequest());
    }

    @Test
    void getTask_returns200() throws Exception {
        TaskDO task = new TaskDO();
        task.setId(1L);
        task.setTenantId(1L);
        task.setUserId(1L);
        task.setRequirement("test");
        task.setStatus("ANALYZING");
        task.setTotalInputTokens(100L);
        task.setTotalOutputTokens(50L);

        when(taskService.getTask(1L)).thenReturn(task);

        mockMvc.perform(get("/api/tasks/1"))
                .andExpect(status().isOk())
                .andExpect(jsonPath("$.data.status").value("ANALYZING"));
    }
}
