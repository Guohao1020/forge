package com.shulex.forge.pipeline.adapter.codeup;

import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.ObjectMapper;
import com.shulex.forge.pipeline.adapter.model.*;
import com.shulex.forge.pipeline.adapter.spi.CodeHostingAdapter;
import lombok.extern.slf4j.Slf4j;
import org.springframework.stereotype.Component;
import java.util.ArrayList;
import java.util.List;

@Slf4j
@Component
public class CodeupAdapter implements CodeHostingAdapter {
    private final CodeupClient client;
    private final ObjectMapper objectMapper;

    public CodeupAdapter(CodeupClient client) {
        this.client = client;
        this.objectMapper = new ObjectMapper();
    }

    @Override public String getType() { return "codeup"; }

    @Override
    public List<FileTreeNode> listRepositoryTree(String repoId, String path, String ref) {
        JsonNode result = client.listRepositoryTree(repoId, path, ref);
        List<FileTreeNode> nodes = new ArrayList<>();
        for (JsonNode node : result) {
            nodes.add(FileTreeNode.builder()
                    .path(node.path("path").asText()).name(node.path("name").asText())
                    .type(node.path("type").asText())
                    .size(node.has("size") ? node.get("size").asLong() : null).build());
        }
        return nodes;
    }

    @Override
    public FileContent getFileContent(String repoId, String filePath, String ref) {
        JsonNode result = client.getFileBlobs(repoId, filePath, ref);
        return FileContent.builder()
                .path(result.path("filePath").asText()).content(result.path("content").asText())
                .encoding(result.path("encoding").asText()).sha(result.path("blobId").asText()).build();
    }

    @Override
    public String createCommitWithMultipleFiles(String repoId, String branch, String commitMessage, List<CommitFile> files) {
        try {
            String actionsJson = objectMapper.writeValueAsString(files);
            JsonNode result = client.createCommit(repoId, branch, commitMessage, actionsJson);
            return result.path("commitId").asText();
        } catch (Exception e) { throw new RuntimeException("提交文件失败", e); }
    }

    @Override
    public BranchInfo createBranch(String repoId, String branchName, String ref) {
        JsonNode result = client.createBranch(repoId, branchName, ref);
        return BranchInfo.builder().name(result.path("name").asText())
                .commitId(result.path("commit").path("id").asText()).build();
    }

    @Override public void deleteBranch(String repoId, String branchName) { client.deleteBranch(repoId, branchName); }

    @Override
    public BranchInfo getBranch(String repoId, String branchName) {
        JsonNode result = client.getBranch(repoId, branchName);
        return BranchInfo.builder().name(result.path("name").asText())
                .commitId(result.path("commit").path("id").asText())
                .isProtected(result.path("protected").asBoolean()).build();
    }

    @Override
    public List<BranchInfo> listBranches(String repoId) {
        JsonNode result = client.listBranches(repoId);
        List<BranchInfo> branches = new ArrayList<>();
        for (JsonNode node : result) {
            branches.add(BranchInfo.builder().name(node.path("name").asText())
                    .commitId(node.path("commit").path("id").asText())
                    .isProtected(node.path("protected").asBoolean()).build());
        }
        return branches;
    }

    @Override
    public MergeRequestInfo createMergeRequest(String repoId, MergeRequestCreateRequest request) {
        JsonNode result = client.createMergeRequest(repoId, request.getTitle(), request.getDescription(),
                request.getSourceBranch(), request.getTargetBranch());
        return parseMergeRequestInfo(result);
    }

    @Override
    public MergeRequestInfo getMergeRequest(String repoId, Long mrId) {
        return parseMergeRequestInfo(client.getMergeRequest(repoId, mrId));
    }

    @Override public void mergeMergeRequest(String repoId, Long mrId) { client.mergeMergeRequest(repoId, mrId); }
    @Override public void closeMergeRequest(String repoId, Long mrId) { client.closeMergeRequest(repoId, mrId); }
    @Override public void addMergeRequestComment(String repoId, Long mrId, String comment) { client.addComment(repoId, mrId, comment); }

    @Override
    public WebhookInfo createWebhook(String repoId, String url, String secretToken, String events) {
        JsonNode result = client.createWebhook(repoId, url, secretToken, events);
        return WebhookInfo.builder().id(result.path("id").asLong()).url(result.path("url").asText())
                .active(result.path("active").asBoolean(true)).secretToken(secretToken).events(events).build();
    }

    @Override
    public List<WebhookInfo> listWebhooks(String repoId) {
        JsonNode result = client.listWebhooks(repoId);
        List<WebhookInfo> webhooks = new ArrayList<>();
        for (JsonNode node : result) {
            webhooks.add(WebhookInfo.builder().id(node.path("id").asLong())
                    .url(node.path("url").asText()).active(node.path("active").asBoolean()).build());
        }
        return webhooks;
    }

    @Override public void deleteWebhook(String repoId, Long webhookId) { client.deleteWebhook(repoId, webhookId); }

    private MergeRequestInfo parseMergeRequestInfo(JsonNode node) {
        return MergeRequestInfo.builder().id(node.path("id").asLong()).title(node.path("title").asText())
                .description(node.path("description").asText()).sourceBranch(node.path("sourceBranch").asText())
                .targetBranch(node.path("targetBranch").asText()).state(node.path("state").asText())
                .url(node.path("webUrl").asText()).build();
    }
}
