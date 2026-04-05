package k8s

import (
	"encoding/json"
	"testing"
)

func TestBuildDockerConfigJSON(t *testing.T) {
	creds := map[string]string{
		"username": "testuser",
		"password": "testpass",
	}

	data, err := buildDockerConfigJSON("registry.example.com", creds)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var config map[string]interface{}
	if err := json.Unmarshal(data, &config); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	auths, ok := config["auths"].(map[string]interface{})
	if !ok {
		t.Fatal("missing auths field")
	}

	regAuth, ok := auths["registry.example.com"].(map[string]interface{})
	if !ok {
		t.Fatal("missing registry entry")
	}

	if regAuth["auth"] == "" {
		t.Error("auth should not be empty")
	}
}

func TestBuildDockerConfigJSON_MissingCreds(t *testing.T) {
	_, err := buildDockerConfigJSON("registry.example.com", map[string]string{})
	if err == nil {
		t.Error("expected error for missing credentials")
	}

	_, err = buildDockerConfigJSON("registry.example.com", map[string]string{"username": "user"})
	if err == nil {
		t.Error("expected error for missing password")
	}
}
