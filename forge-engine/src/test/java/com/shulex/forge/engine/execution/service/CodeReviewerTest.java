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

class CodeReviewerTest {

    private CodeReviewer codeReviewer;
    private ClaudeClient claudeClient;
    private ContextBuilder contextBuilder;

    @BeforeEach
    void setUp() {
        claudeClient = mock(ClaudeClient.class);
        contextBuilder = mock(ContextBuilder.class);
        codeReviewer = new CodeReviewer(claudeClient, contextBuilder);
    }

    @Test
    void review_parsesScoreAndIssues() {
        String aiOutput = "{\"score\": 92, \"issues\": [{\"severity\": \"minor\", \"description\": \"缺少注释\", \"suggestion\": \"添加 Javadoc\"}]}";
        when(contextBuilder.buildSystemPrompt("code-review")).thenReturn("review prompt");
        when(claudeClient.chat(any(), any()))
                .thenReturn(new AiResponse(aiOutput, 50, 30, "claude", "end_turn"));

        List<GeneratedCode> files = List.of(new GeneratedCode("User.java", "code", "CREATE"));
        var result = codeReviewer.review(files);
        assertThat(result.getScore()).isEqualTo(92);
        assertThat(result.getIssues()).hasSize(1);
    }

    @Test
    void review_handlesNonJsonGracefully() {
        when(contextBuilder.buildSystemPrompt("code-review")).thenReturn("prompt");
        when(claudeClient.chat(any(), any()))
                .thenReturn(new AiResponse("The code looks good overall.", 50, 30, "claude", "end_turn"));

        List<GeneratedCode> files = List.of(new GeneratedCode("Test.java", "code", "CREATE"));
        var result = codeReviewer.review(files);
        assertThat(result.getScore()).isEqualTo(80); // default score
    }
}
