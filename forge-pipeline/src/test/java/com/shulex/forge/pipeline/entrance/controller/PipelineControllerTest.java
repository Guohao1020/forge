package com.shulex.forge.pipeline.entrance.controller;

import com.fasterxml.jackson.databind.ObjectMapper;
import com.shulex.forge.pipeline.adapter.spi.AdapterRegistry;
import com.shulex.forge.pipeline.devops.service.WebhookDispatcher;
import com.shulex.forge.pipeline.entrance.vo.TriggerPipelineRequest;
import com.shulex.forge.pipeline.infrastructure.mapper.PipelineExecutionMapper;
import org.junit.jupiter.api.Test;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.boot.test.autoconfigure.web.servlet.AutoConfigureMockMvc;
import org.springframework.boot.test.context.SpringBootTest;
import org.springframework.boot.test.mock.mockito.MockBean;
import org.springframework.data.redis.core.StringRedisTemplate;
import org.springframework.http.MediaType;
import org.springframework.test.context.ActiveProfiles;
import org.springframework.test.web.servlet.MockMvc;

import static org.mockito.Mockito.*;
import static org.springframework.test.web.servlet.request.MockMvcRequestBuilders.*;
import static org.springframework.test.web.servlet.result.MockMvcResultMatchers.*;

@SpringBootTest
@AutoConfigureMockMvc
@ActiveProfiles("test")
class PipelineControllerTest {

    @Autowired
    private MockMvc mockMvc;
    @Autowired
    private ObjectMapper objectMapper;
    @MockBean
    private WebhookDispatcher webhookDispatcher;
    @MockBean
    private PipelineExecutionMapper executionMapper;
    @MockBean
    private StringRedisTemplate redisTemplate;
    @MockBean
    private AdapterRegistry adapterRegistry;

    @Test
    void trigger_returns200() throws Exception {
        TriggerPipelineRequest request = new TriggerPipelineRequest();
        request.setTenantId(1L);
        request.setRepoId("repo-123");
        request.setBranch("main");

        mockMvc.perform(post("/api/pipelines/trigger")
                        .contentType(MediaType.APPLICATION_JSON)
                        .content(objectMapper.writeValueAsString(request)))
                .andExpect(status().isOk());

        verify(webhookDispatcher).onPush(1L, "repo-123", "main", "MANUAL");
    }

    @Test
    void trigger_returns400OnMissingFields() throws Exception {
        mockMvc.perform(post("/api/pipelines/trigger")
                        .contentType(MediaType.APPLICATION_JSON)
                        .content("{}"))
                .andExpect(status().isBadRequest());
    }
}
