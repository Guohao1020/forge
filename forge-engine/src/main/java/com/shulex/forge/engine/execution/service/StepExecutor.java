package com.shulex.forge.engine.execution.service;

import com.fasterxml.jackson.databind.ObjectMapper;
import com.shulex.forge.engine.execution.model.StepRequest;
import com.shulex.forge.engine.execution.model.StepResult;
import com.shulex.forge.engine.orchestration.model.RiskLevel;
import com.shulex.forge.engine.orchestration.model.StepType;
import com.shulex.forge.engine.orchestration.service.RiskAssessor;
import lombok.extern.slf4j.Slf4j;
import org.springframework.stereotype.Service;

import java.util.List;

@Slf4j
@Service
public class StepExecutor {

    private final CodeGenerator codeGenerator;
    private final CodeReviewer codeReviewer;
    private final CodeFixer codeFixer;
    private final CodeCommitter codeCommitter;
    private final ContextBuilder contextBuilder;
    private final RiskAssessor riskAssessor;
    private final com.shulex.forge.engine.execution.ai.ClaudeClient claudeClient;
    private final ObjectMapper objectMapper = new ObjectMapper();

    // 暂存生成的代码文件，按 taskId 隔离，支持并发任务
    private final java.util.concurrent.ConcurrentHashMap<Long, List<com.shulex.forge.engine.execution.model.GeneratedCode>> generatedFilesMap = new java.util.concurrent.ConcurrentHashMap<>();

    public StepExecutor(CodeGenerator codeGenerator, CodeReviewer codeReviewer,
                        CodeFixer codeFixer, CodeCommitter codeCommitter,
                        ContextBuilder contextBuilder, RiskAssessor riskAssessor,
                        com.shulex.forge.engine.execution.ai.ClaudeClient claudeClient) {
        this.codeGenerator = codeGenerator;
        this.codeReviewer = codeReviewer;
        this.codeFixer = codeFixer;
        this.codeCommitter = codeCommitter;
        this.contextBuilder = contextBuilder;
        this.riskAssessor = riskAssessor;
        this.claudeClient = claudeClient;
    }

    public StepResult execute(StepRequest request) {
        StepType stepType = StepType.valueOf(request.getStepType());
        log.info("执行步骤: task={}, step={}, type={}", request.getTaskId(), request.getStepId(), stepType);

        try {
            return switch (stepType) {
                case ANALYZE -> executeAnalyze(request);
                case PLAN -> executePlan(request);
                case RISK_ASSESS_INIT -> executeRiskAssessInit(request);
                case GENERATE_CONTRACT, GENERATE_CODE -> executeGenerate(request);
                case REVIEW -> executeReview(request);
                case RISK_ASSESS_FINAL -> executeRiskAssessFinal(request);
                case COMMIT -> executeCommit(request);
                case CREATE_MR -> executeCreateMR(request);
                case FIX -> executeFix(request);
            };
        } catch (Exception e) {
            log.error("步骤执行失败: task={}, step={}", request.getTaskId(), request.getStepId(), e);
            return new StepResult(request.getTaskId(), request.getStepId(),
                    request.getStepType(), false, null, 0, 0, e.getMessage());
        }
    }

    private StepResult executeAnalyze(StepRequest request) {
        String systemPrompt = contextBuilder.buildSystemPrompt("requirement-analysis");
        if (systemPrompt == null) systemPrompt = "分析以下需求，输出结构化技术任务清单。";
        var response = claudeClient.chat(systemPrompt, request.getRequirement());
        return new StepResult(request.getTaskId(), request.getStepId(),
                request.getStepType(), true, response.getContent(),
                response.getInputTokens(), response.getOutputTokens(), null);
    }

    private StepResult executePlan(StepRequest request) {
        var response = claudeClient.chat("你是技术方案规划师。根据需求分析结果生成实施方案。",
                request.getRequirement());
        return new StepResult(request.getTaskId(), request.getStepId(),
                request.getStepType(), true, response.getContent(),
                response.getInputTokens(), response.getOutputTokens(), null);
    }

    private StepResult executeRiskAssessInit(StepRequest request) {
        RiskLevel risk = riskAssessor.initialAssess(request.getRequirement(), "GENERATE");
        return new StepResult(request.getTaskId(), request.getStepId(),
                request.getStepType(), true, "{\"riskLevel\":\"" + risk.name() + "\"}",
                0, 0, null);
    }

    private StepResult executeGenerate(StepRequest request) {
        var result = codeGenerator.generate(request.getAdapterType(), request.getRepoId(),
                request.getBranchName() != null ? request.getBranchName() : "main",
                request.getRequirement());
        generatedFilesMap.put(request.getTaskId(), result.getFiles());
        String output = result.getFiles().size() + " files generated";
        return new StepResult(request.getTaskId(), request.getStepId(),
                request.getStepType(), true, output,
                result.getInputTokens(), result.getOutputTokens(), null);
    }

    private StepResult executeReview(StepRequest request) {
        var files = generatedFilesMap.get(request.getTaskId());
        if (files == null || files.isEmpty()) {
            return new StepResult(request.getTaskId(), request.getStepId(),
                    request.getStepType(), true, "{\"score\":100,\"issues\":[]}",
                    0, 0, null);
        }
        var result = codeReviewer.review(files);

        int maxFix = 3;
        int round = 0;
        while (result.getScore() < 90 && !result.getIssues().isEmpty() && round < maxFix) {
            round++;
            log.info("自动修复 round {}: score={}", round, result.getScore());
            var fixResult = codeFixer.fix(files, result.getIssues());
            if (!fixResult.getFiles().isEmpty()) {
                files = fixResult.getFiles();
                generatedFilesMap.put(request.getTaskId(), files);
            }
            result = codeReviewer.review(files);
        }

        try {
            String output = objectMapper.writeValueAsString(result);
            return new StepResult(request.getTaskId(), request.getStepId(),
                    request.getStepType(), true, output,
                    result.getInputTokens(), result.getOutputTokens(), null);
        } catch (Exception e) {
            return new StepResult(request.getTaskId(), request.getStepId(),
                    request.getStepType(), true, "{\"score\":" + result.getScore() + "}",
                    result.getInputTokens(), result.getOutputTokens(), null);
        }
    }

    private StepResult executeRiskAssessFinal(StepRequest request) {
        int score = 80;
        var files = generatedFilesMap.get(request.getTaskId());
        int fileCount = files != null ? files.size() : 0;
        RiskLevel risk = riskAssessor.finalAssess(RiskLevel.LOW, score, fileCount);
        return new StepResult(request.getTaskId(), request.getStepId(),
                request.getStepType(), true, "{\"riskLevel\":\"" + risk.name() + "\",\"score\":" + score + "}",
                0, 0, null);
    }

    private StepResult executeCommit(StepRequest request) {
        var files = generatedFilesMap.get(request.getTaskId());
        if (files == null || files.isEmpty()) {
            return new StepResult(request.getTaskId(), request.getStepId(),
                    request.getStepType(), true, "no files to commit", 0, 0, null);
        }
        String branch = codeCommitter.createBranch(request.getAdapterType(), request.getRepoId(), request.getTaskId());
        String commitHash = codeCommitter.commitCode(request.getAdapterType(), request.getRepoId(),
                branch, request.getTaskId(), files);
        return new StepResult(request.getTaskId(), request.getStepId(),
                request.getStepType(), true,
                "{\"branch\":\"" + branch + "\",\"commit\":\"" + commitHash + "\"}",
                0, 0, null);
    }

    private StepResult executeCreateMR(StepRequest request) {
        String branch = "ai/task-" + request.getTaskId();
        Long mrId = codeCommitter.createMergeRequest(request.getAdapterType(), request.getRepoId(),
                branch, request.getTaskId(), request.getRequirement());
        return new StepResult(request.getTaskId(), request.getStepId(),
                request.getStepType(), true,
                "{\"mrId\":" + mrId + "}",
                0, 0, null);
    }

    private StepResult executeFix(StepRequest request) {
        return new StepResult(request.getTaskId(), request.getStepId(),
                request.getStepType(), true, "fix embedded in review", 0, 0, null);
    }
}
