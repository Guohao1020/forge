package com.shulex.forge.engine.infrastructure.mapper;

import com.baomidou.mybatisplus.core.mapper.BaseMapper;
import com.shulex.forge.engine.infrastructure.entity.TaskStepDO;
import org.apache.ibatis.annotations.Mapper;

@Mapper
public interface TaskStepMapper extends BaseMapper<TaskStepDO> {
}
