package com.shulex.forge.specs.service;

import com.baomidou.mybatisplus.core.conditions.query.LambdaQueryWrapper;
import com.shulex.forge.specs.common.BizException;
import com.shulex.forge.specs.common.ErrorCode;
import com.shulex.forge.specs.infrastructure.entity.ScaffoldTemplateDO;
import com.shulex.forge.specs.infrastructure.mapper.ScaffoldTemplateMapper;
import org.springframework.stereotype.Service;
import java.util.List;

@Service
public class ScaffoldTemplateService {

    private final ScaffoldTemplateMapper scaffoldTemplateMapper;

    public ScaffoldTemplateService(ScaffoldTemplateMapper scaffoldTemplateMapper) {
        this.scaffoldTemplateMapper = scaffoldTemplateMapper;
    }

    public ScaffoldTemplateDO getByName(String name) {
        ScaffoldTemplateDO template = scaffoldTemplateMapper.selectOne(
                new LambdaQueryWrapper<ScaffoldTemplateDO>()
                        .eq(ScaffoldTemplateDO::getName, name)
                        .eq(ScaffoldTemplateDO::getIsActive, true)
        );
        if (template == null) {
            throw new BizException(ErrorCode.NOT_FOUND);
        }
        return template;
    }

    public List<ScaffoldTemplateDO> listActive() {
        return scaffoldTemplateMapper.selectList(
                new LambdaQueryWrapper<ScaffoldTemplateDO>()
                        .eq(ScaffoldTemplateDO::getIsActive, true)
        );
    }
}
