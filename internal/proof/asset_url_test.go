package proof

import (
	"os"
	"testing"
)

func TestAssetURLReturnsInputOutsideConfiguredProduction(t *testing.T) {
	tests := []struct {
		name       string
		appEnv     *string
		cdnVersion *string
	}{
		{name: "environment unset"},
		{name: "development", appEnv: stringPointer("development"), cdnVersion: stringPointer("v0.1.2")},
		{name: "production version unset", appEnv: stringPointer("production")},
		{name: "production version empty", appEnv: stringPointer("production"), cdnVersion: stringPointer("")},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			setOptionalEnv(t, "APP_ENV", test.appEnv)
			setOptionalEnv(t, "CDN_VERSION", test.cdnVersion)

			const input = "/assets/icons/brand/sprite.svg#araihu"
			if got := AssetURL(input); got != input {
				t.Fatalf("AssetURL(%q) = %q, want unchanged input", input, got)
			}
		})
	}
}

func TestAssetURLBuildsProductionJsDelivrURL(t *testing.T) {
	t.Setenv("APP_ENV", "production")
	t.Setenv("CDN_VERSION", "v0.1.2")

	tests := []struct {
		name string
		path string
		want string
	}{
		{
			name: "plain relative path",
			path: "styles.css",
			want: "https://cdn.jsdelivr.net/gh/araihu/assets@v0.1.2/dist/proof/styles.css",
		},
		{
			name: "all leading slashes removed",
			path: "///assets/icons/ui/sprite.svg",
			want: "https://cdn.jsdelivr.net/gh/araihu/assets@v0.1.2/dist/proof/assets/icons/ui/sprite.svg",
		},
		{
			name: "query and fragment preserved",
			path: "/assets/icons/ui/sprite.svg?theme=dark#check",
			want: "https://cdn.jsdelivr.net/gh/araihu/assets@v0.1.2/dist/proof/assets/icons/ui/sprite.svg?theme=dark#check",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := AssetURL(test.path); got != test.want {
				t.Fatalf("AssetURL(%q) = %q, want %q", test.path, got, test.want)
			}
		})
	}
}

func TestAssetURLEscapesCDNVersion(t *testing.T) {
	t.Setenv("APP_ENV", "production")
	t.Setenv("CDN_VERSION", "release/v0.1.2")

	const want = "https://cdn.jsdelivr.net/gh/araihu/assets@release%2Fv0.1.2/dist/proof/app.js"
	if got := AssetURL("app.js"); got != want {
		t.Fatalf("AssetURL() = %q, want %q", got, want)
	}
}

func setOptionalEnv(t *testing.T, key string, value *string) {
	t.Helper()
	previous, existed := os.LookupEnv(key)
	if value == nil {
		if err := os.Unsetenv(key); err != nil {
			t.Fatalf("unset %s: %v", key, err)
		}
	} else {
		if err := os.Setenv(key, *value); err != nil {
			t.Fatalf("set %s: %v", key, err)
		}
	}
	t.Cleanup(func() {
		if existed {
			_ = os.Setenv(key, previous)
		} else {
			_ = os.Unsetenv(key)
		}
	})
}

func stringPointer(value string) *string {
	return &value
}
