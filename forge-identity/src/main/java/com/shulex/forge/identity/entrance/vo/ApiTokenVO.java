package com.shulex.forge.identity.entrance.vo;

import lombok.Builder;
import lombok.Data;
import java.time.LocalDateTime;

@Data
@Builder
public class ApiTokenVO {
    private Long id;
    private String tokenName;
    private String tokenPrefix;
    private LocalDateTime expiresAt;
    private Integer status;
    private String rawToken; // 仅创建时返回，列表查询时为 null
}
