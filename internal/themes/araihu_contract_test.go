package themes

import (
	"os"
	"strings"
	"testing"
)

func TestAraiHuThemeUsesModernStructure(t *testing.T) {
	css, err := os.ReadFile("../../themes/araihu.css")
	if err != nil {
		t.Fatal(err)
	}

	content := string(css)
	for _, want := range []string{
		"Modern geometry with the Arai Hû organization palette",
		`--font-body: "Lato", ui-sans-serif, system-ui, sans-serif;`,
		`--font-title: "Lato", ui-sans-serif, system-ui, sans-serif;`,
		"--radius-radius: var(--radius-sm);",
	} {
		if !strings.Contains(content, want) {
			t.Errorf("themes/araihu.css missing Modern contract %q", want)
		}
	}

	for _, obsolete := range []string{"Instrument Sans", "var(--radius-lg)"} {
		if strings.Contains(content, obsolete) {
			t.Errorf("themes/araihu.css retains pre-Modern token %q", obsolete)
		}
	}
}
