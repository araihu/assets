package assetmeta

import (
	"reflect"
	"strings"
	"testing"
)

func TestNewInventoryAcceptsValidResources(t *testing.T) {
	resources := fixtureResources()
	resources = append(resources, Resource{
		Name:    "htmx",
		Version: "2.0.8",
		Downloads: []Download{{
			Name:      "runtime-js",
			URL:       "https://cdn.example/htmx.js",
			Path:      "assets/js/htmx.js",
			Integrity: "sha384-htmx",
			Hash:      "sha384:feedface",
		}},
	})

	inventory, err := NewInventory(resources)
	if err != nil {
		t.Fatalf("NewInventory() error = %v", err)
	}
	if got := inventory.Resources(); !reflect.DeepEqual(got, resources) {
		t.Fatalf("Resources() = %#v, want %#v", got, resources)
	}
	got, ok := inventory.Resource("htmx")
	if !ok || !reflect.DeepEqual(got, resources[1]) {
		t.Fatalf("Resource(htmx) = %#v, %v", got, ok)
	}
	if _, ok := inventory.Resource("missing"); ok {
		t.Fatal("Resource(missing) unexpectedly resolved")
	}
}

func TestNewInventoryRejectsInvalidResources(t *testing.T) {
	tests := []struct {
		name      string
		resources []Resource
		want      string
	}{
		{name: "empty resource name", resources: []Resource{{Version: "1", Downloads: validDownloads()}}, want: "resource name is empty"},
		{name: "empty version", resources: []Resource{{Name: "lib", Downloads: validDownloads()}}, want: `resource "lib": version is empty`},
		{name: "no downloads", resources: []Resource{{Name: "lib", Version: "1"}}, want: `resource "lib": downloads are empty`},
		{name: "duplicate resource", resources: []Resource{
			{Name: "lib", Version: "1", Downloads: validDownloads()},
			{Name: "lib", Version: "2", Downloads: validDownloads()},
		}, want: `resource "lib": duplicate name`},
		{name: "empty download name", resources: []Resource{{Name: "lib", Version: "1", Downloads: []Download{{URL: "u", Path: "p", Integrity: "i", Hash: "h"}}}}, want: `resource "lib": download name is empty`},
		{name: "duplicate download", resources: []Resource{{Name: "lib", Version: "1", Downloads: []Download{
			{Name: "file", URL: "u", Path: "p", Integrity: "i", Hash: "h"},
			{Name: "file", URL: "u2", Path: "p2", Integrity: "i2", Hash: "h2"},
		}}}, want: `resource "lib" download "file": duplicate name`},
		{name: "empty URL", resources: []Resource{{Name: "lib", Version: "1", Downloads: []Download{{Name: "file", Path: "p", Integrity: "i", Hash: "h"}}}}, want: `resource "lib" download "file": URL is empty`},
		{name: "empty path", resources: []Resource{{Name: "lib", Version: "1", Downloads: []Download{{Name: "file", URL: "u", Integrity: "i", Hash: "h"}}}}, want: `resource "lib" download "file": path is empty`},
		{name: "empty integrity", resources: []Resource{{Name: "lib", Version: "1", Downloads: []Download{{Name: "file", URL: "u", Path: "p", Hash: "h"}}}}, want: `resource "lib" download "file": integrity is empty`},
		{name: "empty hash", resources: []Resource{{Name: "lib", Version: "1", Downloads: []Download{{Name: "file", URL: "u", Path: "p", Integrity: "i"}}}}, want: `resource "lib" download "file": hash is empty`},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := NewInventory(test.resources)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("NewInventory() error = %v, want containing %q", err, test.want)
			}
		})
	}
}

func TestInventoryOwnsResourceCopies(t *testing.T) {
	input := fixtureResources()
	inventory, err := NewInventory(input)
	if err != nil {
		t.Fatal(err)
	}

	input[0].Version = "changed"
	input[0].Downloads[0].Hash = "changed"
	got, ok := inventory.Resource("alpinejs")
	if !ok || got.Version != "3.14.9" || got.Downloads[0].Hash != "sha384:0123456789abcdef" {
		t.Fatalf("input mutation reached inventory: %#v", got)
	}

	listed := inventory.Resources()
	listed[0].Version = "changed again"
	listed[0].Downloads[0].Hash = "changed again"
	got, _ = inventory.Resource("alpinejs")
	if got.Version != "3.14.9" || got.Downloads[0].Hash != "sha384:0123456789abcdef" {
		t.Fatalf("Resources mutation reached inventory: %#v", got)
	}

	got.Version = "changed once more"
	got.Downloads[0].Hash = "changed once more"
	again, _ := inventory.Resource("alpinejs")
	if again.Version != "3.14.9" || again.Downloads[0].Hash != "sha384:0123456789abcdef" {
		t.Fatalf("Resource mutation reached inventory: %#v", again)
	}
}

func TestInventoryPreservesOpaqueHashes(t *testing.T) {
	resources := []Resource{{
		Name:    "digests",
		Version: "1",
		Downloads: []Download{
			{Name: "sha256", URL: "https://example/256", Path: "256", Integrity: "sha256-sri", Hash: "sha256:AbCd"},
			{Name: "sha384", URL: "https://example/384", Path: "384", Integrity: "sha384-sri", Hash: "sha384:0123"},
			{Name: "sha512", URL: "https://example/512", Path: "512", Integrity: "sha512-sri", Hash: "sha512:FeDc"},
		},
	}}
	inventory, err := NewInventory(resources)
	if err != nil {
		t.Fatal(err)
	}
	got, _ := inventory.Resource("digests")
	for index, want := range []string{"sha256:AbCd", "sha384:0123", "sha512:FeDc"} {
		if got.Downloads[index].Hash != want {
			t.Fatalf("download %d hash = %q, want %q", index, got.Downloads[index].Hash, want)
		}
	}
}

func TestInventoryNilReceiverIsEmpty(t *testing.T) {
	var inventory *Inventory
	if got := inventory.Resources(); got != nil {
		t.Fatalf("Resources() = %#v, want nil", got)
	}
	if _, ok := inventory.Resource("anything"); ok {
		t.Fatal("nil Inventory resolved resource")
	}
}

func fixtureResources() []Resource {
	return []Resource{{
		Name:    "alpinejs",
		Version: "3.14.9",
		Downloads: []Download{{
			Name:      "core-js",
			URL:       "https://cdn.example/alpine.js",
			Path:      "assets/js/alpine.js",
			Integrity: "sha384-base64-value",
			Hash:      "sha384:0123456789abcdef",
		}},
	}}
}

func validDownloads() []Download {
	return []Download{{Name: "file", URL: "u", Path: "p", Integrity: "i", Hash: "h"}}
}
