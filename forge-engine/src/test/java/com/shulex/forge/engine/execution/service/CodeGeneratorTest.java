package com.shulex.forge.engine.execution.service;

import com.shulex.forge.engine.execution.ai.AiResponse;
import com.shulex.forge.engine.execution.ai.ClaudeClient;
import com.shulex.forge.engine.execution.model.GeneratedCode;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;

import java.util.List;

import static org.assertj.core.api.Assertions.assertThat;
import static org.mockito.ArgumentMatchers.any;
import static org.mockito.Mockito.*;

class CodeGeneratorTest {

    private CodeGenerator codeGenerator;
    private ClaudeClient claudeClient;
    private ContextBuilder contextBuilder;

    @BeforeEach
    void setUp() {
        claudeClient = mock(ClaudeClient.class);
        contextBuilder = mock(ContextBuilder.class);
        codeGenerator = new CodeGenerator(claudeClient, contextBuilder);
    }

    @Test
    void generate_parsesFileBlocks() {
        String aiOutput = "```file:src/main/java/User.java\npublic class User {}\n```\n"
                + "```file:src/main/java/UserService.java\npublic class UserService {}\n```";
        when(contextBuilder.buildSystemPrompt("code-generation")).thenReturn("sys prompt");
        when(contextBuilder.buildContext(any(), any(), any(), any())).thenReturn("context");
        when(claudeClient.chat(any(), any()))
                .thenReturn(new AiResponse(aiOutput, 100, 200, "claude", "end_turn"));

        var result = codeGenerator.generate("codeup", "repo", "main", "创建用户服务");
        assertThat(result.getFiles()).hasSize(2);
        assertThat(result.getFiles().get(0).getFilePath()).isEqualTo("src/main/java/User.java");
        assertThat(result.getInputTokens()).isEqualTo(100);
    }

    @Test
    void generate_handlesEmptyResponse() {
        when(contextBuilder.buildSystemPrompt(any())).thenReturn("sys");
        when(contextBuilder.buildContext(any(), any(), any(), any())).thenReturn("ctx");
        when(claudeClient.chat(any(), any()))
                .thenReturn(new AiResponse("No code needed.", 10, 5, "claude", "end_turn"));

        var result = codeGenerator.generate("codeup", "repo", "main", "test");
        assertThat(result.getFiles()).isEmpty();
    }
}
