package com.shulex.forge.specs.infrastructure.mapper;

import com.baomidou.mybatisplus.core.mapper.BaseMapper;
import com.shulex.forge.specs.infrastructure.entity.ScaffoldTemplateDO;
import org.apache.ibatis.annotations.Mapper;

@Mapper
public interface ScaffoldTemplateMapper extends BaseMapper<ScaffoldTemplateDO> {
}
