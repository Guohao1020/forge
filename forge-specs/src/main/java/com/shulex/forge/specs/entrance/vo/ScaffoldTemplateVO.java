package com.shulex.forge.specs.entrance.vo;

import lombok.Data;

@Data
public class ScaffoldTemplateVO {
    private Long id;
    private String name;
    private String description;
    private String techStack;
    private String templateContent;
}
