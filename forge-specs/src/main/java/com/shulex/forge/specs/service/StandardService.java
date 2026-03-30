package com.shulex.forge.specs.service;

import com.baomidou.mybatisplus.core.conditions.query.LambdaQueryWrapper;
import com.shulex.forge.specs.infrastructure.entity.StandardDO;
import com.shulex.forge.specs.infrastructure.mapper.StandardMapper;
import org.springframework.stereotype.Service;
import java.util.List;

@Service
public class StandardService {

    private final StandardMapper standardMapper;

    public StandardService(StandardMapper standardMapper) {
        this.standardMapper = standardMapper;
    }

    public List<StandardDO> listByCategory(String category) {
        return standardMapper.selectList(
                new LambdaQueryWrapper<StandardDO>()
                        .eq(StandardDO::getCategory, category)
                        .eq(StandardDO::getIsEnabled, true)
                        .orderByAsc(StandardDO::getSortOrder)
        );
    }

    public List<StandardDO> listEffective(String category, String scopeLevel, String scopeId) {
        return standardMapper.selectList(
                new LambdaQueryWrapper<StandardDO>()
                        .eq(StandardDO::getCategory, category)
                        .eq(StandardDO::getIsEnabled, true)
                        .and(w -> w
                                .eq(StandardDO::getScopeLevel, "company")
                                .or()
                                .eq(StandardDO::getScopeLevel, scopeLevel)
                                .eq(scopeId != null, StandardDO::getScopeId, scopeId)
                        )
                        .orderByAsc(StandardDO::getSortOrder)
        );
    }
}
