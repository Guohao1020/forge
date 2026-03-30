package com.shulex.forge.pipeline.infrastructure.mapper;

import com.baomidou.mybatisplus.core.mapper.BaseMapper;
import com.shulex.forge.pipeline.infrastructure.entity.EnvironmentDO;
import org.apache.ibatis.annotations.Mapper;

@Mapper
public interface EnvironmentMapper extends BaseMapper<EnvironmentDO> {}
