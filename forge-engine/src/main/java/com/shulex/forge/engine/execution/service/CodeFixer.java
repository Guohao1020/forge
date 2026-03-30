package com.shulex.forge.engine.execution.service;

import com.shulex.forge.engine.execution.ai.AiResponse;
import com.shulex.forge.engine.execution.ai.ClaudeClient;
import com.shulex.forge.engine.execution.model.GeneratedCode;
import lombok.extern.slf4j.Slf4j;
import org.springframework.stereotype.Service;

import java.util.List;
import java.util.regex.Matcher;
import java.util.regex.Pattern;
import java.util.ArrayList;

@Slf4j
@Service
public class CodeFixer {

    private static final Pattern FILE_BLOCK = Pattern.compile(
            "```file:([^\\n]+)\\n(.*?)```", Pattern.DOTALL);

    private final ClaudeClient claudeClient;
    private final ContextBuilder contextBuilder;

    public CodeFixer(ClaudeClient claudeClient, ContextBuilder contextBuilder) {
        this.claudeClient = claudeClient;
        this.contextBuilder = contextBuilder;
    }

    public CodeGenerator.GenerateResult fix(List<GeneratedCode> originalFiles,
                                             List<CodeReviewer.ReviewIssue> issues) {
        String systemPrompt = contextBuilder.buildSystemPrompt("code-fix");
        StringBuilder userMessage = new StringBuilder("请修复以下代码中的问题：\n\n");

        userMessage.append("# 问题列表\n\n");
        for (CodeReviewer.ReviewIssue issue : issues) {
            userMessage.append("- [").append(issue.getSeverity()).append("] ")
                    .append(issue.getDescription());
            if (issue.getSuggestion() != null && !issue.getSuggestion().isBlank()) {
                userMessage.append(" → ").append(issue.getSuggestion());
            }
            userMessage.append("\n");
        }

        userMessage.append("\n# 原始代码\n\n");
        for (GeneratedCode file : originalFiles) {
            userMessage.append("## ").append(file.getFilePath()).append("\n```\n")
                    .append(file.getContent()).append("\n```\n\n");
        }

        AiResponse response = claudeClient.chat(systemPrompt, userMessage.toString());

        List<GeneratedCode> fixedFiles = parseFixedFiles(response.getContent(), originalFiles);
        log.info("代码修复完成: fixedFiles={}", fixedFiles.size());
        return new CodeGenerator.GenerateResult(fixedFiles, response.getInputTokens(),
                response.getOutputTokens(), response.getContent());
    }

    private List<GeneratedCode> parseFixedFiles(String content, List<GeneratedCode> originals) {
        Matcher matcher = FILE_BLOCK.matcher(content);
        List<GeneratedCode> files = new ArrayList<>();
        while (matcher.find()) {
            files.add(new GeneratedCode(matcher.group(1).trim(), matcher.group(2).trim(), "MODIFY"));
        }
        return files.isEmpty() ? originals : files;
    }
}
