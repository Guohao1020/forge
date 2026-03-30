package com.shulex.forge.pipeline.adapter.ack;

import com.shulex.forge.pipeline.adapter.model.DeploymentInfo;
import com.shulex.forge.pipeline.adapter.model.PodInfo;
import io.kubernetes.client.openapi.ApiException;
import io.kubernetes.client.openapi.apis.AppsV1Api;
import io.kubernetes.client.openapi.apis.CoreV1Api;
import io.kubernetes.client.openapi.models.*;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;
import org.mockito.Mockito;

import java.time.OffsetDateTime;
import java.util.List;
import java.util.Map;

import static org.assertj.core.api.Assertions.assertThat;
import static org.mockito.ArgumentMatchers.any;
import static org.mockito.Mockito.*;

class AckAdapterTest {
    private CoreV1Api coreV1Api;
    private AppsV1Api appsV1Api;
    private AckAdapter adapter;

    @BeforeEach
    void setUp() {
        coreV1Api = Mockito.mock(CoreV1Api.class);
        appsV1Api = Mockito.mock(AppsV1Api.class);
        K8sClientFactory factory = Mockito.mock(K8sClientFactory.class);
        when(factory.coreV1Api()).thenReturn(coreV1Api);
        when(factory.appsV1Api()).thenReturn(appsV1Api);
        adapter = new AckAdapter(factory);
    }

    @Test
    void getType_returnsAck() {
        assertThat(adapter.getType()).isEqualTo("ack");
    }

    @Test
    void createNamespace_callsCoreApi() throws Exception {
        CoreV1Api.APIcreateNamespaceRequest request = Mockito.mock(CoreV1Api.APIcreateNamespaceRequest.class);
        when(coreV1Api.createNamespace(any(V1Namespace.class))).thenReturn(request);
        when(request.execute()).thenReturn(new V1Namespace());

        adapter.createNamespace("test-ns", Map.of("env", "dev"));

        verify(coreV1Api).createNamespace(any(V1Namespace.class));
        verify(request).execute();
    }

    @Test
    void namespaceExists_returnsTrueWhenFound() throws Exception {
        CoreV1Api.APIreadNamespaceRequest request = Mockito.mock(CoreV1Api.APIreadNamespaceRequest.class);
        when(coreV1Api.readNamespace("test-ns")).thenReturn(request);
        when(request.execute()).thenReturn(new V1Namespace());

        assertThat(adapter.namespaceExists("test-ns")).isTrue();
    }

    @Test
    void namespaceExists_returnsFalseOnNotFound() throws Exception {
        CoreV1Api.APIreadNamespaceRequest request = Mockito.mock(CoreV1Api.APIreadNamespaceRequest.class);
        when(coreV1Api.readNamespace("missing")).thenReturn(request);
        when(request.execute()).thenThrow(new ApiException(404, "not found"));

        assertThat(adapter.namespaceExists("missing")).isFalse();
    }

    @Test
    void getDeployment_convertsToModel() throws Exception {
        V1Deployment k8sDeploy = new V1Deployment()
                .metadata(new V1ObjectMeta().name("my-app").namespace("dev"))
                .spec(new V1DeploymentSpec().replicas(3)
                        .template(new V1PodTemplateSpec()
                                .spec(new V1PodSpec().containers(List.of(
                                        new V1Container().name("my-app").image("my-image:latest"))))))
                .status(new V1DeploymentStatus().availableReplicas(3));

        AppsV1Api.APIreadNamespacedDeploymentRequest request = Mockito.mock(AppsV1Api.APIreadNamespacedDeploymentRequest.class);
        when(appsV1Api.readNamespacedDeployment("my-app", "dev")).thenReturn(request);
        when(request.execute()).thenReturn(k8sDeploy);

        DeploymentInfo info = adapter.getDeployment("dev", "my-app");
        assertThat(info.getName()).isEqualTo("my-app");
        assertThat(info.getReplicas()).isEqualTo(3);
        assertThat(info.getAvailableReplicas()).isEqualTo(3);
    }

    @Test
    void listPods_convertsToModel() throws Exception {
        V1PodList podList = new V1PodList().items(List.of(
                new V1Pod()
                        .metadata(new V1ObjectMeta().name("my-app-abc123").namespace("dev"))
                        .status(new V1PodStatus().phase("Running").startTime(OffsetDateTime.now()))
                        .spec(new V1PodSpec().nodeName("node-1"))
        ));

        CoreV1Api.APIlistNamespacedPodRequest request = Mockito.mock(CoreV1Api.APIlistNamespacedPodRequest.class);
        when(coreV1Api.listNamespacedPod("dev")).thenReturn(request);
        when(request.labelSelector("app=my-app")).thenReturn(request);
        when(request.execute()).thenReturn(podList);

        List<PodInfo> pods = adapter.listPods("dev", Map.of("app", "my-app"));
        assertThat(pods).hasSize(1);
        assertThat(pods.get(0).getPhase()).isEqualTo("Running");
    }
}
