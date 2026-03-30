package com.shulex.forge.pipeline.entrance.controller;

import com.fasterxml.jackson.databind.JsonNode;
import com.shulex.forge.pipeline.common.Result;
import com.shulex.forge.pipeline.devops.service.WebhookDispatcher;
import lombok.extern.slf4j.Slf4j;
import org.springframework.web.bind.annotation.*;

@Slf4j
@RestController
@RequestMapping("/api/webhooks")
public class WebhookController {

    private final WebhookDispatcher webhookDispatcher;

    public WebhookController(WebhookDispatcher webhookDispatcher) {
        this.webhookDispatcher = webhookDispatcher;
    }

    @PostMapping("/push")
    public Result<Void> onPush(@RequestBody JsonNode payload) {
        log.info("收到 Webhook push 事件");
        String repoId = payload.path("repository").path("id").asText();
        String branch = payload.path("ref").asText("").replace("refs/heads/", "");
        Long tenantId = payload.path("tenantId").asLong(1L);

        if (!branch.isBlank()) {
            webhookDispatcher.onPush(tenantId, repoId, branch, "WEBHOOK");
        }
        return Result.ok(null);
    }

    @PostMapping("/merge-request")
    public Result<Void> onMergeRequest(@RequestBody JsonNode payload) {
        log.info("收到 Webhook MR 事件");
        String action = payload.path("action").asText();
        if ("merge".equals(action)) {
            String repoId = payload.path("repository").path("id").asText();
            String sourceBranch = payload.path("merge_request").path("source_branch").asText();
            String targetBranch = payload.path("merge_request").path("target_branch").asText();
            Long tenantId = payload.path("tenantId").asLong(1L);

            webhookDispatcher.onMergeRequestMerged(tenantId, repoId, sourceBranch, targetBranch);
        }
        return Result.ok(null);
    }
}
