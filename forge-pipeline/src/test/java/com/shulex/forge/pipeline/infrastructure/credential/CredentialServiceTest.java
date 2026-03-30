package com.shulex.forge.pipeline.infrastructure.credential;

import org.junit.jupiter.api.Test;
import org.springframework.core.env.StandardEnvironment;

import static org.assertj.core.api.Assertions.assertThat;
import static org.assertj.core.api.Assertions.assertThatThrownBy;

class CredentialServiceTest {

    @Test
    void getCredential_throwsWhenMissing() {
        CredentialService service = new CredentialService(new StandardEnvironment());
        assertThatThrownBy(() -> service.getCredential("nonexistent.key"))
                .isInstanceOf(IllegalStateException.class)
                .hasMessageContaining("凭证未配置");
    }

    @Test
    void getCredential_returnsDefaultWhenMissing() {
        CredentialService service = new CredentialService(new StandardEnvironment());
        String result = service.getCredential("nonexistent.key", "default-val");
        assertThat(result).isEqualTo("default-val");
    }
}
