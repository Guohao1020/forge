package artifact

import (
	"encoding/json"
	"testing"
)

func TestArtifactStruct(t *testing.T) {
	size := int64(1024 * 1024 * 50) // 50MB
	a := Artifact{
		ProjectID:    1,
		Name:         "forge-core",
		Version:      "v1.0.0",
		ArtifactType: "docker_image",
		SizeBytes:    &size,
		Status:       "READY",
	}
	if a.ArtifactType != "docker_image" {
		t.Errorf("expected docker_image, got %s", a.ArtifactType)
	}
	if *a.SizeBytes != 50*1024*1024 {
		t.Errorf("expected 50MB, got %d", *a.SizeBytes)
	}
}

func TestArtifactMetadata(t *testing.T) {
	meta := json.RawMessage(`{"digest":"sha256:abc123","layers":5}`)
	a := Artifact{
		Name:     "test",
		Metadata: meta,
	}
	if len(a.Metadata) == 0 {
		t.Error("metadata should not be empty")
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal(a.Metadata, &parsed); err != nil {
		t.Fatalf("metadata parse error: %v", err)
	}
	if parsed["digest"] != "sha256:abc123" {
		t.Errorf("expected digest sha256:abc123, got %v", parsed["digest"])
	}
}
