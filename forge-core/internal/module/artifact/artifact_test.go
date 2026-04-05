package artifact

import (
	"encoding/json"
	"testing"
)

func TestArtifactTypes(t *testing.T) {
	types := []string{"docker_image", "npm_package", "apk", "ipa", "exe", "dmg"}
	for _, at := range types {
		a := Artifact{ArtifactType: at}
		if a.ArtifactType == "" {
			t.Errorf("type should not be empty")
		}
	}
}

func TestArtifactListResponse_Empty(t *testing.T) {
	resp := ArtifactListResponse{Artifacts: []Artifact{}}
	if len(resp.Artifacts) != 0 {
		t.Error("expected empty")
	}
}

func TestArtifactJSON(t *testing.T) {
	size := int64(1024)
	a := Artifact{
		Name:         "forge-core",
		Version:      "v1.0.0",
		ArtifactType: "docker_image",
		SizeBytes:    &size,
		Status:       "READY",
	}
	data, err := json.Marshal(a)
	if err != nil {
		t.Fatal(err)
	}
	var parsed Artifact
	json.Unmarshal(data, &parsed)
	if parsed.Name != "forge-core" {
		t.Errorf("expected forge-core, got %s", parsed.Name)
	}
}
