package com.shulex.forge.specs.entrance.vo;

import lombok.Data;

@Data
public class PromptTemplateVO {
    private Long id;
    private String templateKey;
    private String name;
    private String description;
    private String systemPrompt;
    private String standardsInjection;
    private Integer version;
}
