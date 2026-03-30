package com.shulex.forge.pipeline.infrastructure.mapper;

import com.baomidou.mybatisplus.core.mapper.BaseMapper;
import com.shulex.forge.pipeline.infrastructure.entity.DeploymentRecordDO;
import org.apache.ibatis.annotations.Mapper;

@Mapper
public interface DeploymentRecordMapper extends BaseMapper<DeploymentRecordDO> {}
