package acquisition

import (
	"os"
	"testing"
)

func TestIconPackInventoryIsComplete(t *testing.T) {
	wantVersions := map[string]string{"heroicons": "v2.2.0", "developer-icons": "v7.0.1"}
	if got := len(MuambaResources()); got != len(wantVersions) {
		t.Fatalf("resources = %d, want %d", got, len(wantVersions))
	}
	for name, version := range wantVersions {
		resource, ok := MuambaResourceByName(name)
		if !ok {
			t.Fatalf("resource %q missing", name)
		}
		if resource.Version != version {
			t.Fatalf("%s version = %q, want %q", name, resource.Version, version)
		}
		if len(resource.Downloads) != 1 {
			t.Fatalf("%s downloads = %d, want license only", name, len(resource.Downloads))
		}
		seen := map[string]bool{}
		for _, download := range resource.Downloads {
			if download.Integrity == "" || download.Hash == "" {
				t.Fatalf("unlocked %s/%s", name, download.Name)
			}
			seen[download.Name] = true
		}
		if !seen["license"] {
			t.Fatalf("%s downloads = %#v", name, seen)
		}
	}

	source, err := Repository(os.DirFS("../.."), ".muamba.lock.yaml")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []struct {
		resource  string
		directory string
		files     int
	}{
		{resource: "heroicons", directory: "optimized", files: 1288},
		{resource: "developer-icons", directory: "icons", files: 318},
	} {
		locked, ok := source.Directory(want.resource, want.directory)
		if !ok {
			t.Fatalf("directory %s/%s missing", want.resource, want.directory)
		}
		if len(locked.Files) != want.files {
			t.Fatalf("directory %s/%s files = %d, want %d", want.resource, want.directory, len(locked.Files), want.files)
		}
		if locked.Integrity == "" || locked.URL == "" {
			t.Fatalf("directory %s/%s is unlocked", want.resource, want.directory)
		}
	}
}
