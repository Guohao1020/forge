package com.shulex.forge.specs.service;

import com.shulex.forge.specs.common.BizException;
import com.shulex.forge.specs.infrastructure.entity.ScaffoldTemplateDO;
import com.shulex.forge.specs.infrastructure.mapper.ScaffoldTemplateMapper;
import org.junit.jupiter.api.Test;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.boot.test.context.SpringBootTest;
import org.springframework.test.context.ActiveProfiles;
import org.springframework.transaction.annotation.Transactional;
import java.util.List;
import static org.assertj.core.api.Assertions.assertThat;
import static org.assertj.core.api.Assertions.assertThatThrownBy;

@SpringBootTest
@ActiveProfiles("test")
@Transactional
class ScaffoldTemplateServiceTest {

    @Autowired
    private ScaffoldTemplateService scaffoldTemplateService;

    @Autowired
    private ScaffoldTemplateMapper scaffoldTemplateMapper;

    @Test
    void getByName_returnsTemplate() {
        ScaffoldTemplateDO t = new ScaffoldTemplateDO();
        t.setName("java-microservice");
        t.setDescription("Java 微服务骨架");
        t.setTechStack("Java 17, Spring Boot 3.2, MyBatis Plus");
        t.setTemplateContent("{\"files\":[]}");
        t.setIsActive(true);
        scaffoldTemplateMapper.insert(t);

        ScaffoldTemplateDO result = scaffoldTemplateService.getByName("java-microservice");
        assertThat(result.getDescription()).isEqualTo("Java 微服务骨架");
    }

    @Test
    void getByName_throwsWhenNotFound() {
        assertThatThrownBy(() -> scaffoldTemplateService.getByName("nonexistent"))
                .isInstanceOf(BizException.class);
    }

    @Test
    void listActive_returnsOnlyActive() {
        ScaffoldTemplateDO active = new ScaffoldTemplateDO();
        active.setName("active-scaffold");
        active.setTemplateContent("{}");
        active.setIsActive(true);
        scaffoldTemplateMapper.insert(active);

        ScaffoldTemplateDO inactive = new ScaffoldTemplateDO();
        inactive.setName("inactive-scaffold");
        inactive.setTemplateContent("{}");
        inactive.setIsActive(false);
        scaffoldTemplateMapper.insert(inactive);

        List<ScaffoldTemplateDO> result = scaffoldTemplateService.listActive();
        assertThat(result.stream().noneMatch(s -> "inactive-scaffold".equals(s.getName()))).isTrue();
    }
}
