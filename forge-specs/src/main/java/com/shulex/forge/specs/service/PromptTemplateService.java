package com.shulex.forge.specs.service;

import com.baomidou.mybatisplus.core.conditions.query.LambdaQueryWrapper;
import com.shulex.forge.specs.common.BizException;
import com.shulex.forge.specs.common.ErrorCode;
import com.shulex.forge.specs.infrastructure.entity.PromptTemplateDO;
import com.shulex.forge.specs.infrastructure.mapper.PromptTemplateMapper;
import org.springframework.stereotype.Service;
import java.util.List;

@Service
public class PromptTemplateService {

    private final PromptTemplateMapper promptTemplateMapper;

    public PromptTemplateService(PromptTemplateMapper promptTemplateMapper) {
        this.promptTemplateMapper = promptTemplateMapper;
    }

    public PromptTemplateDO getActiveByKey(String templateKey) {
        PromptTemplateDO template = promptTemplateMapper.selectOne(
                new LambdaQueryWrapper<PromptTemplateDO>()
                        .eq(PromptTemplateDO::getTemplateKey, templateKey)
                        .eq(PromptTemplateDO::getIsActive, true)
        );
        if (template == null) {
            throw new BizException(ErrorCode.NOT_FOUND);
        }
        return template;
    }

    public List<PromptTemplateDO> listActive() {
        return promptTemplateMapper.selectList(
                new LambdaQueryWrapper<PromptTemplateDO>()
                        .eq(PromptTemplateDO::getIsActive, true)
        );
    }
}
