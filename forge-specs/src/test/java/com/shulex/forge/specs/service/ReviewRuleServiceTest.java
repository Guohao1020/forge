package com.shulex.forge.specs.service;

import com.shulex.forge.specs.infrastructure.entity.ReviewRuleDO;
import com.shulex.forge.specs.infrastructure.mapper.ReviewRuleMapper;
import org.junit.jupiter.api.Test;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.boot.test.context.SpringBootTest;
import org.springframework.test.context.ActiveProfiles;
import java.util.List;
import org.springframework.transaction.annotation.Transactional;
import static org.assertj.core.api.Assertions.assertThat;

@SpringBootTest
@ActiveProfiles("test")
@Transactional
class ReviewRuleServiceTest {

    @Autowired
    private ReviewRuleService reviewRuleService;

    @Autowired
    private ReviewRuleMapper reviewRuleMapper;

    @Test
    void listByCategory_returnsEnabledRules() {
        ReviewRuleDO rule = new ReviewRuleDO();
        rule.setCategory("security");
        rule.setRuleKey("sql-injection-check");
        rule.setName("SQL 注入检查");
        rule.setDescription("检查是否使用参数化查询");
        rule.setSeverity("error");
        rule.setIsEnabled(true);
        reviewRuleMapper.insert(rule);

        List<ReviewRuleDO> result = reviewRuleService.listByCategory("security");
        assertThat(result).hasSize(1);
        assertThat(result.get(0).getRuleKey()).isEqualTo("sql-injection-check");
    }

    @Test
    void listAll_returnsAllEnabledRules() {
        ReviewRuleDO r1 = new ReviewRuleDO();
        r1.setCategory("coding");
        r1.setRuleKey("naming-convention");
        r1.setName("命名规范检查");
        r1.setDescription("...");
        r1.setSeverity("warning");
        r1.setIsEnabled(true);
        reviewRuleMapper.insert(r1);

        List<ReviewRuleDO> result = reviewRuleService.listAllEnabled();
        assertThat(result).isNotEmpty();
    }
}
