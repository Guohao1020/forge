package com.shulex.forge.specs.infrastructure.mapper;

import com.baomidou.mybatisplus.core.mapper.BaseMapper;
import com.shulex.forge.specs.infrastructure.entity.ReviewRuleDO;
import org.apache.ibatis.annotations.Mapper;

@Mapper
public interface ReviewRuleMapper extends BaseMapper<ReviewRuleDO> {
}
