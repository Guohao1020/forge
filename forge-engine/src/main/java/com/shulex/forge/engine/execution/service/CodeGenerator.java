package com.shulex.forge.engine.execution.service;

import com.shulex.forge.engine.execution.ai.AiResponse;
import com.shulex.forge.engine.execution.ai.ClaudeClient;
import com.shulex.forge.engine.execution.model.GeneratedCode;
import lombok.AllArgsConstructor;
import lombok.Data;
import lombok.extern.slf4j.Slf4j;
import org.springframework.stereotype.Service;

import java.util.ArrayList;
import java.util.List;
import java.util.regex.Matcher;
import java.util.regex.Pattern;

@Slf4j
@Service
public class CodeGenerator {

    private static final Pattern FILE_BLOCK = Pattern.compile(
            "```file:([^\\n]+)\\n(.*?)```", Pattern.DOTALL);

    private final ClaudeClient claudeClient;
    private final ContextBuilder contextBuilder;

    public CodeGenerator(ClaudeClient claudeClient, ContextBuilder contextBuilder) {
        this.claudeClient = claudeClient;
        this.contextBuilder = contextBuilder;
    }

    public GenerateResult generate(String adapterType, String repoId, String ref, String requirement) {
        String systemPrompt = contextBuilder.buildSystemPrompt("code-generation");
        String context = contextBuilder.buildContext(adapterType, repoId, ref, requirement);
        String userMessage = "# 需求\n\n" + requirement + "\n\n# 项目上下文\n\n" + context;

        AiResponse response = claudeClient.chat(systemPrompt, userMessage);
        List<GeneratedCode> files = parseFiles(response.getContent());

        log.info("代码生成完成: files={}, tokens={}+{}", files.size(),
                response.getInputTokens(), response.getOutputTokens());

        return new GenerateResult(files, response.getInputTokens(), response.getOutputTokens(), response.getContent());
    }

    private List<GeneratedCode> parseFiles(String content) {
        List<GeneratedCode> files = new ArrayList<>();
        Matcher matcher = FILE_BLOCK.matcher(content);
        while (matcher.find()) {
            String filePath = matcher.group(1).trim();
            String fileContent = matcher.group(2).trim();
            files.add(new GeneratedCode(filePath, fileContent, "CREATE"));
        }
        return files;
    }

    @Data
    @AllArgsConstructor
    public static class GenerateResult {
        private List<GeneratedCode> files;
        private long inputTokens;
        private long outputTokens;
        private String rawResponse;
    }
}
