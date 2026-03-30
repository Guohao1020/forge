package com.shulex.forge.engine.execution.service;

import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.ObjectMapper;
import com.shulex.forge.engine.execution.ai.AiResponse;
import com.shulex.forge.engine.execution.ai.ClaudeClient;
import com.shulex.forge.engine.execution.model.GeneratedCode;
import lombok.AllArgsConstructor;
import lombok.Data;
import lombok.NoArgsConstructor;
import lombok.extern.slf4j.Slf4j;
import org.springframework.stereotype.Service;

import java.util.ArrayList;
import java.util.List;

@Slf4j
@Service
public class CodeReviewer {

    private final ClaudeClient claudeClient;
    private final ContextBuilder contextBuilder;
    private final ObjectMapper objectMapper = new ObjectMapper();

    public CodeReviewer(ClaudeClient claudeClient, ContextBuilder contextBuilder) {
        this.claudeClient = claudeClient;
        this.contextBuilder = contextBuilder;
    }

    public ReviewResult review(List<GeneratedCode> files) {
        String systemPrompt = contextBuilder.buildSystemPrompt("code-review");
        StringBuilder userMessage = new StringBuilder("请审查以下代码：\n\n");
        for (GeneratedCode file : files) {
            userMessage.append("## ").append(file.getFilePath()).append("\n```\n")
                    .append(file.getContent()).append("\n```\n\n");
        }

        AiResponse response = claudeClient.chat(systemPrompt, userMessage.toString());
        return parseReviewResult(response);
    }

    private ReviewResult parseReviewResult(AiResponse response) {
        try {
            String content = response.getContent().trim();
            if (content.contains("```json")) {
                content = content.substring(content.indexOf("```json") + 7);
                content = content.substring(0, content.indexOf("```"));
            } else if (content.contains("```")) {
                content = content.substring(content.indexOf("```") + 3);
                content = content.substring(0, content.indexOf("```"));
            }
            content = content.trim();

            JsonNode root = objectMapper.readTree(content);
            int score = root.path("score").asInt(80);
            List<ReviewIssue> issues = new ArrayList<>();
            for (JsonNode issue : root.path("issues")) {
                issues.add(new ReviewIssue(
                        issue.path("severity").asText("minor"),
                        issue.path("description").asText(),
                        issue.path("suggestion").asText("")
                ));
            }
            return new ReviewResult(score, issues, response.getInputTokens(), response.getOutputTokens());
        } catch (Exception e) {
            log.warn("解析 Review 结果失败，使用默认评分: {}", e.getMessage());
            return new ReviewResult(80, List.of(), response.getInputTokens(), response.getOutputTokens());
        }
    }

    @Data
    @AllArgsConstructor
    public static class ReviewResult {
        private int score;
        private List<ReviewIssue> issues;
        private long inputTokens;
        private long outputTokens;
    }

    @Data
    @NoArgsConstructor
    @AllArgsConstructor
    public static class ReviewIssue {
        private String severity;
        private String description;
        private String suggestion;
    }
}
