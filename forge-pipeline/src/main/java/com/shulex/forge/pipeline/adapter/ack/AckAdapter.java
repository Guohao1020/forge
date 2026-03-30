package com.shulex.forge.pipeline.adapter.ack;

import com.shulex.forge.pipeline.adapter.model.DeploymentInfo;
import com.shulex.forge.pipeline.adapter.model.PodInfo;
import com.shulex.forge.pipeline.adapter.model.ServiceInfo;
import com.shulex.forge.pipeline.adapter.spi.ContainerOrchestrationAdapter;
import io.kubernetes.client.custom.IntOrString;
import io.kubernetes.client.openapi.ApiException;
import io.kubernetes.client.openapi.apis.AppsV1Api;
import io.kubernetes.client.openapi.apis.CoreV1Api;
import io.kubernetes.client.openapi.models.*;
import lombok.extern.slf4j.Slf4j;
import org.springframework.stereotype.Component;

import java.util.List;
import java.util.Map;
import java.util.stream.Collectors;

@Slf4j
@Component
public class AckAdapter implements ContainerOrchestrationAdapter {
    private final K8sClientFactory clientFactory;

    public AckAdapter(K8sClientFactory clientFactory) {
        this.clientFactory = clientFactory;
    }

    @Override
    public String getType() { return "ack"; }

    @Override
    public void createNamespace(String namespace, Map<String, String> labels) {
        try {
            V1Namespace ns = new V1Namespace()
                    .metadata(new V1ObjectMeta().name(namespace).labels(labels));
            clientFactory.coreV1Api().createNamespace(ns).execute();
            log.info("创建 Namespace: {}", namespace);
        } catch (ApiException e) {
            throw new RuntimeException("创建 Namespace 失败: " + namespace, e);
        }
    }

    @Override
    public void deleteNamespace(String namespace) {
        try {
            clientFactory.coreV1Api().deleteNamespace(namespace).execute();
        } catch (ApiException e) {
            throw new RuntimeException("删除 Namespace 失败: " + namespace, e);
        }
    }

    @Override
    public boolean namespaceExists(String namespace) {
        try {
            clientFactory.coreV1Api().readNamespace(namespace).execute();
            return true;
        } catch (ApiException e) {
            if (e.getCode() == 404) return false;
            throw new RuntimeException("查询 Namespace 失败: " + namespace, e);
        }
    }

    @Override
    public DeploymentInfo createOrUpdateDeployment(String namespace, String name, String image,
                                                    Integer replicas, Map<String, String> env) {
        try {
            V1Container container = new V1Container().name(name).image(image);
            if (env != null) {
                env.forEach((k, v) -> container.addEnvItem(new V1EnvVar().name(k).value(v)));
            }

            V1Deployment deployment = new V1Deployment()
                    .metadata(new V1ObjectMeta().name(name).namespace(namespace))
                    .spec(new V1DeploymentSpec().replicas(replicas)
                            .selector(new V1LabelSelector().matchLabels(Map.of("app", name)))
                            .template(new V1PodTemplateSpec()
                                    .metadata(new V1ObjectMeta().labels(Map.of("app", name)))
                                    .spec(new V1PodSpec().containers(List.of(container)))));

            V1Deployment result;
            try {
                clientFactory.appsV1Api().readNamespacedDeployment(name, namespace).execute();
                result = clientFactory.appsV1Api()
                        .replaceNamespacedDeployment(name, namespace, deployment).execute();
            } catch (ApiException e) {
                if (e.getCode() == 404) {
                    result = clientFactory.appsV1Api()
                            .createNamespacedDeployment(namespace, deployment).execute();
                } else {
                    throw e;
                }
            }
            return toDeploymentInfo(result);
        } catch (ApiException e) {
            throw new RuntimeException("创建/更新 Deployment 失败", e);
        }
    }

    @Override
    public DeploymentInfo getDeployment(String namespace, String name) {
        try {
            return toDeploymentInfo(
                    clientFactory.appsV1Api().readNamespacedDeployment(name, namespace).execute());
        } catch (ApiException e) {
            throw new RuntimeException("查询 Deployment 失败", e);
        }
    }

    @Override
    public void deleteDeployment(String namespace, String name) {
        try {
            clientFactory.appsV1Api().deleteNamespacedDeployment(name, namespace).execute();
        } catch (ApiException e) {
            throw new RuntimeException("删除 Deployment 失败", e);
        }
    }

    @Override
    public void scaleDeployment(String namespace, String name, int replicas) {
        try {
            V1Deployment existing = clientFactory.appsV1Api()
                    .readNamespacedDeployment(name, namespace).execute();
            existing.getSpec().setReplicas(replicas);
            clientFactory.appsV1Api()
                    .replaceNamespacedDeployment(name, namespace, existing).execute();
        } catch (ApiException e) {
            throw new RuntimeException("扩缩 Deployment 失败", e);
        }
    }

    @Override
    public List<PodInfo> listPods(String namespace, Map<String, String> labelSelector) {
        try {
            String selector = labelSelector.entrySet().stream()
                    .map(e -> e.getKey() + "=" + e.getValue())
                    .collect(Collectors.joining(","));
            V1PodList podList = clientFactory.coreV1Api()
                    .listNamespacedPod(namespace)
                    .labelSelector(selector)
                    .execute();
            return podList.getItems().stream().map(this::toPodInfo).toList();
        } catch (ApiException e) {
            throw new RuntimeException("查询 Pod 列表失败", e);
        }
    }

    @Override
    public String getPodLogs(String namespace, String podName, int tailLines) {
        try {
            return clientFactory.coreV1Api()
                    .readNamespacedPodLog(podName, namespace)
                    .tailLines(tailLines)
                    .execute();
        } catch (ApiException e) {
            throw new RuntimeException("获取 Pod 日志失败", e);
        }
    }

    @Override
    public ServiceInfo createOrUpdateService(String namespace, String name, String type,
                                              Map<String, String> selector, int port, int targetPort) {
        try {
            V1Service service = new V1Service()
                    .metadata(new V1ObjectMeta().name(name).namespace(namespace))
                    .spec(new V1ServiceSpec().type(type).selector(selector)
                            .ports(List.of(new V1ServicePort()
                                    .port(port)
                                    .targetPort(new IntOrString(targetPort)))));
            V1Service result;
            try {
                clientFactory.coreV1Api().readNamespacedService(name, namespace).execute();
                result = clientFactory.coreV1Api()
                        .replaceNamespacedService(name, namespace, service).execute();
            } catch (ApiException e) {
                if (e.getCode() == 404) {
                    result = clientFactory.coreV1Api()
                            .createNamespacedService(namespace, service).execute();
                } else {
                    throw e;
                }
            }
            return toServiceInfo(result);
        } catch (ApiException e) {
            throw new RuntimeException("创建/更新 Service 失败", e);
        }
    }

    @Override
    public ServiceInfo getService(String namespace, String name) {
        try {
            return toServiceInfo(
                    clientFactory.coreV1Api().readNamespacedService(name, namespace).execute());
        } catch (ApiException e) {
            throw new RuntimeException("查询 Service 失败", e);
        }
    }

    @Override
    public void deleteService(String namespace, String name) {
        try {
            clientFactory.coreV1Api().deleteNamespacedService(name, namespace).execute();
        } catch (ApiException e) {
            throw new RuntimeException("删除 Service 失败", e);
        }
    }

    @Override
    public void createOrUpdateConfigMap(String namespace, String name, Map<String, String> data) {
        try {
            V1ConfigMap configMap = new V1ConfigMap()
                    .metadata(new V1ObjectMeta().name(name).namespace(namespace))
                    .data(data);
            try {
                clientFactory.coreV1Api().readNamespacedConfigMap(name, namespace).execute();
                clientFactory.coreV1Api()
                        .replaceNamespacedConfigMap(name, namespace, configMap).execute();
            } catch (ApiException e) {
                if (e.getCode() == 404) {
                    clientFactory.coreV1Api()
                            .createNamespacedConfigMap(namespace, configMap).execute();
                } else {
                    throw e;
                }
            }
        } catch (ApiException e) {
            throw new RuntimeException("创建/更新 ConfigMap 失败", e);
        }
    }

    @Override
    public void deleteConfigMap(String namespace, String name) {
        try {
            clientFactory.coreV1Api().deleteNamespacedConfigMap(name, namespace).execute();
        } catch (ApiException e) {
            throw new RuntimeException("删除 ConfigMap 失败", e);
        }
    }

    private DeploymentInfo toDeploymentInfo(V1Deployment d) {
        V1DeploymentStatus status = d.getStatus();
        String statusStr = "progressing";
        if (status != null && status.getAvailableReplicas() != null
                && status.getAvailableReplicas().equals(d.getSpec().getReplicas())) {
            statusStr = "available";
        }
        String image = "";
        if (d.getSpec().getTemplate().getSpec() != null
                && !d.getSpec().getTemplate().getSpec().getContainers().isEmpty()) {
            image = d.getSpec().getTemplate().getSpec().getContainers().get(0).getImage();
        }
        return DeploymentInfo.builder()
                .namespace(d.getMetadata().getNamespace())
                .name(d.getMetadata().getName())
                .image(image)
                .replicas(d.getSpec().getReplicas())
                .availableReplicas(status != null ? status.getAvailableReplicas() : 0)
                .status(statusStr)
                .build();
    }

    private PodInfo toPodInfo(V1Pod pod) {
        return PodInfo.builder()
                .namespace(pod.getMetadata().getNamespace())
                .name(pod.getMetadata().getName())
                .phase(pod.getStatus().getPhase())
                .nodeName(pod.getSpec().getNodeName())
                .startTime(pod.getStatus().getStartTime() != null
                        ? pod.getStatus().getStartTime().toLocalDateTime() : null)
                .build();
    }

    private ServiceInfo toServiceInfo(V1Service s) {
        V1ServiceSpec spec = s.getSpec();
        return ServiceInfo.builder()
                .namespace(s.getMetadata().getNamespace())
                .name(s.getMetadata().getName())
                .type(spec.getType())
                .selector(spec.getSelector())
                .port(spec.getPorts() != null && !spec.getPorts().isEmpty()
                        ? spec.getPorts().get(0).getPort() : null)
                .targetPort(spec.getPorts() != null && !spec.getPorts().isEmpty()
                        && spec.getPorts().get(0).getTargetPort() != null
                        ? spec.getPorts().get(0).getTargetPort().getIntValue() : null)
                .clusterIp(spec.getClusterIP())
                .build();
    }
}
