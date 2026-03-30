package com.shulex.forge.pipeline.adapter.model;
import lombok.Builder;
import lombok.Data;
@Data
@Builder
public class WebhookInfo {
    private Long id;
    private String url;
    private boolean active;
    private String secretToken;
    private String events;
}
