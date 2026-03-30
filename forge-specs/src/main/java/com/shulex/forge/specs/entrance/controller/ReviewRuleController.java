package com.shulex.forge.specs.entrance.controller;

import com.shulex.forge.specs.common.Result;
import com.shulex.forge.specs.entrance.vo.ReviewRuleVO;
import com.shulex.forge.specs.infrastructure.entity.ReviewRuleDO;
import com.shulex.forge.specs.service.ReviewRuleService;
import org.springframework.web.bind.annotation.*;
import java.util.List;

@RestController
@RequestMapping("/api/review-rules")
public class ReviewRuleController {

    private final ReviewRuleService reviewRuleService;

    public ReviewRuleController(ReviewRuleService reviewRuleService) {
        this.reviewRuleService = reviewRuleService;
    }

    @GetMapping
    public Result<List<ReviewRuleVO>> listByCategory(@RequestParam(value = "category") String category) {
        List<ReviewRuleDO> list = reviewRuleService.listByCategory(category);
        return Result.ok(list.stream().map(this::toVO).toList());
    }

    @GetMapping("/all")
    public Result<List<ReviewRuleVO>> listAll() {
        List<ReviewRuleDO> list = reviewRuleService.listAllEnabled();
        return Result.ok(list.stream().map(this::toVO).toList());
    }

    private ReviewRuleVO toVO(ReviewRuleDO entity) {
        ReviewRuleVO vo = new ReviewRuleVO();
        vo.setId(entity.getId());
        vo.setCategory(entity.getCategory());
        vo.setRuleKey(entity.getRuleKey());
        vo.setName(entity.getName());
        vo.setDescription(entity.getDescription());
        vo.setSeverity(entity.getSeverity());
        return vo;
    }
}
