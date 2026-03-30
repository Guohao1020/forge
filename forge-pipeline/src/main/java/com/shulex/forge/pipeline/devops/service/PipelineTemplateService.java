package com.shulex.forge.pipeline.devops.service;

import com.shulex.forge.pipeline.devops.model.ProjectType;
import lombok.extern.slf4j.Slf4j;
import org.springframework.stereotype.Service;

@Slf4j
@Service
public class PipelineTemplateService {

    public String generateTemplate(ProjectType projectType, String projectName, String branch) {
        return switch (projectType) {
            case JAVA_SERVICE -> generateJavaServiceTemplate(projectName, branch);
            case VUE_FRONTEND -> generateVueFrontendTemplate(projectName, branch);
            case SDK_LIBRARY -> generateSdkLibraryTemplate(projectName, branch);
        };
    }

    private String generateJavaServiceTemplate(String projectName, String branch) {
        return """
                name: %s-pipeline
                trigger:
                  branch: %s
                stages:
                  - name: compile
                    steps:
                      - run: mvn clean compile -q
                  - name: unit-test
                    steps:
                      - run: mvn test
                  - name: image-build
                    steps:
                      - run: docker build -t registry.cn-hangzhou.aliyuncs.com/forge/%s:${BUILD_NUMBER} .
                      - run: docker push registry.cn-hangzhou.aliyuncs.com/forge/%s:${BUILD_NUMBER}
                  - name: deploy
                    steps:
                      - run: echo "deploy triggered"
                """.formatted(projectName, branch, projectName, projectName);
    }

    private String generateVueFrontendTemplate(String projectName, String branch) {
        return """
                name: %s-pipeline
                trigger:
                  branch: %s
                stages:
                  - name: install
                    steps:
                      - run: npm install
                  - name: build
                    steps:
                      - run: npm run build
                  - name: image-build
                    steps:
                      - run: docker build -t registry.cn-hangzhou.aliyuncs.com/forge/%s:${BUILD_NUMBER} .
                      - run: docker push registry.cn-hangzhou.aliyuncs.com/forge/%s:${BUILD_NUMBER}
                """.formatted(projectName, branch, projectName, projectName);
    }

    private String generateSdkLibraryTemplate(String projectName, String branch) {
        return """
                name: %s-pipeline
                trigger:
                  branch: %s
                stages:
                  - name: compile
                    steps:
                      - run: mvn clean compile -q
                  - name: unit-test
                    steps:
                      - run: mvn test
                  - name: publish
                    steps:
                      - run: mvn deploy -DskipTests
                """.formatted(projectName, branch);
    }
}
