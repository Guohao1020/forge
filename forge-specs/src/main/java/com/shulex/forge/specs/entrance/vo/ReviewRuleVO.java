package com.shulex.forge.specs.entrance.vo;

import lombok.Data;

@Data
public class ReviewRuleVO {
    private Long id;
    private String category;
    private String ruleKey;
    private String name;
    private String description;
    private String severity;
}
