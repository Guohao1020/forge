package com.shulex.forge.pipeline.adapter.spi;

import com.shulex.forge.pipeline.adapter.model.DeploymentInfo;
import com.shulex.forge.pipeline.adapter.model.PodInfo;
import com.shulex.forge.pipeline.adapter.model.ServiceInfo;
import java.util.List;
import java.util.Map;

public interface ContainerOrchestrationAdapter {
    String getType();
    void createNamespace(String namespace, Map<String, String> labels);
    void deleteNamespace(String namespace);
    boolean namespaceExists(String namespace);
    DeploymentInfo createOrUpdateDeployment(String namespace, String name, String image, Integer replicas, Map<String, String> env);
    DeploymentInfo getDeployment(String namespace, String name);
    void deleteDeployment(String namespace, String name);
    void scaleDeployment(String namespace, String name, int replicas);
    List<PodInfo> listPods(String namespace, Map<String, String> labelSelector);
    String getPodLogs(String namespace, String podName, int tailLines);
    ServiceInfo createOrUpdateService(String namespace, String name, String type, Map<String, String> selector, int port, int targetPort);
    ServiceInfo getService(String namespace, String name);
    void deleteService(String namespace, String name);
    void createOrUpdateConfigMap(String namespace, String name, Map<String, String> data);
    void deleteConfigMap(String namespace, String name);
}
