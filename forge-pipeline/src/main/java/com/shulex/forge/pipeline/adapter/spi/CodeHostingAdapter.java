package com.shulex.forge.pipeline.adapter.spi;

import com.shulex.forge.pipeline.adapter.model.*;
import java.util.List;

public interface CodeHostingAdapter {
    String getType();
    List<FileTreeNode> listRepositoryTree(String repoId, String path, String ref);
    FileContent getFileContent(String repoId, String filePath, String ref);
    String createCommitWithMultipleFiles(String repoId, String branch, String commitMessage, List<CommitFile> files);
    BranchInfo createBranch(String repoId, String branchName, String ref);
    void deleteBranch(String repoId, String branchName);
    BranchInfo getBranch(String repoId, String branchName);
    List<BranchInfo> listBranches(String repoId);
    MergeRequestInfo createMergeRequest(String repoId, MergeRequestCreateRequest request);
    MergeRequestInfo getMergeRequest(String repoId, Long mrId);
    void mergeMergeRequest(String repoId, Long mrId);
    void closeMergeRequest(String repoId, Long mrId);
    void addMergeRequestComment(String repoId, Long mrId, String comment);
    WebhookInfo createWebhook(String repoId, String url, String secretToken, String events);
    List<WebhookInfo> listWebhooks(String repoId);
    void deleteWebhook(String repoId, Long webhookId);
}
