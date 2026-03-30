package com.shulex.forge.engine.execution.model;

import lombok.AllArgsConstructor;
import lombok.Data;
import lombok.NoArgsConstructor;

@Data
@NoArgsConstructor
@AllArgsConstructor
public class GeneratedCode {
    private String filePath;
    private String content;
    private String action; // CREATE, MODIFY, DELETE
}
