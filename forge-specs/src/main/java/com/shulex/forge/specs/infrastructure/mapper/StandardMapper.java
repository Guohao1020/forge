package com.shulex.forge.specs.infrastructure.mapper;

import com.baomidou.mybatisplus.core.mapper.BaseMapper;
import com.shulex.forge.specs.infrastructure.entity.StandardDO;
import org.apache.ibatis.annotations.Mapper;

@Mapper
public interface StandardMapper extends BaseMapper<StandardDO> {
}
