package com.shulex.forge.engine.execution.service;

import com.shulex.forge.engine.execution.model.GeneratedCode;
import com.shulex.forge.engine.infrastructure.entity.CodeChangeDO;
import com.shulex.forge.engine.infrastructure.http.PipelineClient;
import com.shulex.forge.engine.infrastructure.mapper.CodeChangeMapper;
import lombok.extern.slf4j.Slf4j;
import org.springframework.stereotype.Service;

import java.util.List;

@Slf4j
@Service
public class CodeCommitter {

    private final PipelineClient pipelineClient;
    private final CodeChangeMapper codeChangeMapper;

    public CodeCommitter(PipelineClient pipelineClient, CodeChangeMapper codeChangeMapper) {
        this.pipelineClient = pipelineClient;
        this.codeChangeMapper = codeChangeMapper;
    }

    public String createBranch(String adapterType, String repoId, Long taskId) {
        String branchName = "ai/task-" + taskId;
        String result = pipelineClient.createBranch(adapterType, repoId, branchName, "main");
        log.info("创建分支: {}", branchName);
        return branchName;
    }

    public String commitCode(String adapterType, String repoId, String branch,
                              Long taskId, List<GeneratedCode> files) {
        String commitMessage = "[forge-ai] task-" + taskId + ": 自动生成代码";
        String commitHash = pipelineClient.commitFiles(adapterType, repoId, branch, commitMessage, files);

        CodeChangeDO change = new CodeChangeDO();
        change.setTaskId(taskId);
        change.setRepoId(repoId);
        change.setBranchName(branch);
        change.setCommitHash(commitHash);
        change.setFileCount(files.size());
        codeChangeMapper.insert(change);

        log.info("代码提交成功: task={}, commit={}, files={}", taskId, commitHash, files.size());
        return commitHash;
    }

    public Long createMergeRequest(String adapterType, String repoId, String branch,
                                    Long taskId, String requirement) {
        String title = "[forge-ai] task-" + taskId;
        String description = "## AI 生成代码\n\n**需求:** " + requirement;
        Long mrId = pipelineClient.createMergeRequest(adapterType, repoId, branch, "main", title, description);

        if (mrId != null) {
            CodeChangeDO change = new CodeChangeDO();
            change.setTaskId(taskId);
            change.setRepoId(repoId);
            change.setBranchName(branch);
            change.setMrId(mrId);
            change.setMrStatus("OPEN");
            change.setFileCount(0);
            codeChangeMapper.insert(change);
        }

        log.info("创建 MR: task={}, mrId={}", taskId, mrId);
        return mrId;
    }
}
