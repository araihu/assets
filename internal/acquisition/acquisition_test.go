package acquisition

import "testing"

func TestHeroiconsInventoryIsComplete(t *testing.T) {
	resource, ok := MuambaResourceByName("heroicons")
	if !ok {
		t.Fatal("heroicons resource missing")
	}
	if resource.Version != "v2.2.0" {
		t.Fatalf("version = %q", resource.Version)
	}
	if len(resource.Downloads) != 68 {
		t.Fatalf("downloads = %d, want 67 icons plus license", len(resource.Downloads))
	}
	seenLicense := false
	for _, download := range resource.Downloads {
		if download.Integrity == "" || download.Hash == "" {
			t.Fatalf("unlocked %s", download.Name)
		}
		if download.Name == "license" {
			seenLicense = true
		}
	}
	if !seenLicense {
		t.Fatal("license download missing")
	}
}
