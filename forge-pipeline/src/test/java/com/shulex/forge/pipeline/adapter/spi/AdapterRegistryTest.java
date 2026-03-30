package com.shulex.forge.pipeline.adapter.spi;

import com.shulex.forge.pipeline.adapter.model.*;
import org.junit.jupiter.api.Test;

import java.util.List;
import java.util.Map;

import static org.assertj.core.api.Assertions.assertThat;
import static org.assertj.core.api.Assertions.assertThatThrownBy;

class AdapterRegistryTest {

    @Test
    void registersAndRetrievesAdapters() {
        CodeHostingAdapter mockCodeHosting = new CodeHostingAdapter() {
            @Override public String getType() { return "test-code"; }
            @Override public List<FileTreeNode> listRepositoryTree(String r, String p, String ref) { return List.of(); }
            @Override public FileContent getFileContent(String r, String f, String ref) { return null; }
            @Override public String createCommitWithMultipleFiles(String r, String b, String m, List<CommitFile> f) { return ""; }
            @Override public BranchInfo createBranch(String r, String b, String ref) { return null; }
            @Override public void deleteBranch(String r, String b) {}
            @Override public BranchInfo getBranch(String r, String b) { return null; }
            @Override public List<BranchInfo> listBranches(String r) { return List.of(); }
            @Override public MergeRequestInfo createMergeRequest(String r, MergeRequestCreateRequest req) { return null; }
            @Override public MergeRequestInfo getMergeRequest(String r, Long id) { return null; }
            @Override public void mergeMergeRequest(String r, Long id) {}
            @Override public void closeMergeRequest(String r, Long id) {}
            @Override public void addMergeRequestComment(String r, Long id, String c) {}
            @Override public WebhookInfo createWebhook(String r, String u, String s, String e) { return null; }
            @Override public List<WebhookInfo> listWebhooks(String r) { return List.of(); }
            @Override public void deleteWebhook(String r, Long id) {}
        };

        AdapterRegistry registry = new AdapterRegistry(
                List.of(mockCodeHosting), List.of(), List.of());

        assertThat(registry.getCodeHostingAdapter("test-code")).isNotNull();

        Map<String, List<String>> types = registry.getRegisteredAdapterTypes();
        assertThat(types.get("codeHosting")).contains("test-code");
    }

    @Test
    void throwsOnUnknownAdapter() {
        AdapterRegistry registry = new AdapterRegistry(List.of(), List.of(), List.of());

        assertThatThrownBy(() -> registry.getCodeHostingAdapter("nonexistent"))
                .isInstanceOf(IllegalArgumentException.class);
    }
}
