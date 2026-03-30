package com.shulex.forge.specs.service;

import com.shulex.forge.specs.infrastructure.entity.StandardDO;
import com.shulex.forge.specs.infrastructure.mapper.StandardMapper;
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
class StandardServiceTest {

    @Autowired
    private StandardService standardService;

    @Autowired
    private StandardMapper standardMapper;

    @Test
    void listByCategory_returnsMatchingStandards() {
        StandardDO std = new StandardDO();
        std.setCategory("java");
        std.setTitle("命名规范");
        std.setContent("类名使用 UpperCamelCase");
        std.setScopeLevel("company");
        std.setSortOrder(1);
        std.setIsEnabled(true);
        standardMapper.insert(std);

        List<StandardDO> result = standardService.listByCategory("java");
        assertThat(result).hasSize(1);
        assertThat(result.get(0).getTitle()).isEqualTo("命名规范");
    }

    @Test
    void listByCategory_excludesDisabled() {
        StandardDO std = new StandardDO();
        std.setCategory("sql");
        std.setTitle("已禁用规范");
        std.setContent("...");
        std.setScopeLevel("company");
        std.setSortOrder(1);
        std.setIsEnabled(false);
        standardMapper.insert(std);

        List<StandardDO> result = standardService.listByCategory("sql");
        assertThat(result).isEmpty();
    }

    @Test
    void listEffective_mergesScopes() {
        StandardDO company = new StandardDO();
        company.setCategory("java");
        company.setTitle("公司命名规范");
        company.setContent("公司级");
        company.setScopeLevel("company");
        company.setSortOrder(1);
        company.setIsEnabled(true);
        standardMapper.insert(company);

        StandardDO project = new StandardDO();
        project.setCategory("java");
        project.setTitle("项目命名规范");
        project.setContent("项目级覆盖");
        project.setScopeLevel("project");
        project.setScopeId("proj-001");
        project.setSortOrder(1);
        project.setIsEnabled(true);
        standardMapper.insert(project);

        List<StandardDO> result = standardService.listEffective("java", "project", "proj-001");
        assertThat(result).hasSize(2);
    }
}
