package com.shulex.forge.specs.service;

import com.baomidou.mybatisplus.core.conditions.query.LambdaQueryWrapper;
import com.shulex.forge.specs.infrastructure.entity.ReviewRuleDO;
import com.shulex.forge.specs.infrastructure.mapper.ReviewRuleMapper;
import org.springframework.stereotype.Service;
import java.util.List;

@Service
public class ReviewRuleService {

    private final ReviewRuleMapper reviewRuleMapper;

    public ReviewRuleService(ReviewRuleMapper reviewRuleMapper) {
        this.reviewRuleMapper = reviewRuleMapper;
    }

    public List<ReviewRuleDO> listByCategory(String category) {
        return reviewRuleMapper.selectList(
                new LambdaQueryWrapper<ReviewRuleDO>()
                        .eq(ReviewRuleDO::getCategory, category)
                        .eq(ReviewRuleDO::getIsEnabled, true)
        );
    }

    public List<ReviewRuleDO> listAllEnabled() {
        return reviewRuleMapper.selectList(
                new LambdaQueryWrapper<ReviewRuleDO>()
                        .eq(ReviewRuleDO::getIsEnabled, true)
        );
    }
}
