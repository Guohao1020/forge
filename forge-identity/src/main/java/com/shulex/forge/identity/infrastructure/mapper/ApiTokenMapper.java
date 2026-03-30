package com.shulex.forge.identity.infrastructure.mapper;

import com.baomidou.mybatisplus.core.mapper.BaseMapper;
import com.shulex.forge.identity.infrastructure.entity.ApiTokenDO;
import org.apache.ibatis.annotations.Mapper;

@Mapper
public interface ApiTokenMapper extends BaseMapper<ApiTokenDO> {}
