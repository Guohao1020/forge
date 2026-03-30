package com.shulex.forge.engine.execution.service;

import com.shulex.forge.engine.infrastructure.http.PipelineClient;
import com.shulex.forge.engine.infrastructure.http.SpecsClient;
import lombok.extern.slf4j.Slf4j;
import org.springframework.stereotype.Service;

import java.util.List;

@Slf4j
@Service
public class ContextBuilder {

    private final SpecsClient specsClient;
    private final PipelineClient pipelineClient;

    public ContextBuilder(SpecsClient specsClient, PipelineClient pipelineClient) {
        this.specsClient = specsClient;
        this.pipelineClient = pipelineClient;
    }

    public String buildContext(String adapterType, String repoId, String ref, String requirement) {
        StringBuilder context = new StringBuilder();

        // 1. 编码规范
        String standards = specsClient.getStandards("java");
        if (standards != null && !standards.isBlank()) {
            context.append("# 编码规范\n\n").append(standards).append("\n\n");
        }

        // 2. Review 规则
        String rules = specsClient.getReviewRules();
        if (rules != null && !rules.isBlank()) {
            context.append("# Review 规则\n\n").append(rules).append("\n\n");
        }

        // 3. 项目文件结构
        List<String> files = pipelineClient.listRepositoryTree(adapterType, repoId, "/", ref);
        if (!files.isEmpty()) {
            context.append("# 项目文件结构\n\n");
            for (String file : files) {
                context.append("- ").append(file).append("\n");
            }
            context.append("\n");
        }

        log.info("上下文构建完成: 长度={}", context.length());
        return context.toString();
    }

    public String buildSystemPrompt(String templateKey) {
        String template = specsClient.getPromptTemplate(templateKey);
        if (template != null) {
            return template;
        }
        return getDefaultSystemPrompt(templateKey);
    }

    private String getDefaultSystemPrompt(String templateKey) {
        return switch (templateKey) {
            case "code-generation" -> "你是一个资深 Java 开发工程师。根据需求和项目上下文生成高质量的生产级代码。"
                    + "输出格式：每个文件用 ```file:路径 ``` 包裹。";
            case "code-review" -> "你是一个代码审查专家。审查代码是否符合编码规范、安全性和最佳实践。"
                    + "输出格式：JSON {\"score\": 0-100, \"issues\": [{\"severity\": \"...\", \"description\": \"...\", \"suggestion\": \"...\"}]}";
            case "code-fix" -> "你是一个代码修复专家。根据 Review 反馈修复代码问题。"
                    + "输出格式：每个文件用 ```file:路径 ``` 包裹。";
            default -> "你是一个 AI 编程助手。";
        };
    }
}
