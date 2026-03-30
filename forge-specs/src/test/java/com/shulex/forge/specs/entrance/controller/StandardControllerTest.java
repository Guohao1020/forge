package com.shulex.forge.specs.entrance.controller;

import org.junit.jupiter.api.Test;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.boot.test.autoconfigure.web.servlet.AutoConfigureMockMvc;
import org.springframework.boot.test.context.SpringBootTest;
import org.springframework.test.context.ActiveProfiles;
import org.springframework.test.web.servlet.MockMvc;
import static org.springframework.test.web.servlet.request.MockMvcRequestBuilders.get;
import static org.springframework.test.web.servlet.result.MockMvcResultMatchers.*;

@SpringBootTest
@AutoConfigureMockMvc
@ActiveProfiles("test")
class StandardControllerTest {

    @Autowired
    private MockMvc mockMvc;

    @Test
    void listByCategory_returns200() throws Exception {
        mockMvc.perform(get("/api/standards").param("category", "java"))
                .andExpect(status().isOk())
                .andExpect(jsonPath("$.code").value(0))
                .andExpect(jsonPath("$.data").isArray());
    }

    @Test
    void listEffective_returns200() throws Exception {
        mockMvc.perform(get("/api/standards/effective")
                        .param("category", "java")
                        .param("scopeLevel", "project")
                        .param("scopeId", "proj-001"))
                .andExpect(status().isOk())
                .andExpect(jsonPath("$.code").value(0));
    }
}
