package manifest

import "testing"

func TestProductPaintAndMonochromeOverridesAreClosed(t *testing.T) {
	for _, mode := range []string{"missing appearance", "invalid color", "changed source", "half monochrome"} {
		t.Run(mode, func(t *testing.T) {
			m := validBrand(t)
			var index int
			for i, p := range m.Products {
				if p.ID == "oie" {
					index = i
				}
			}
			p := &m.Products[index]
			switch mode {
			case "missing appearance":
				delete(p.Paint, "dark")
			case "invalid color":
				p.Paint["light"]["#6092ab"] = "url(https://invalid)"
			case "changed source":
				delete(p.Paint["dark"], "#6092ab")
				p.Paint["dark"]["#abcdef"] = "#ffffff"
			case "half monochrome":
				delete(p.Sources["original"], "logo-monochrome")
				delete(p.SourceHashes, "logo-monochrome")
			}
			if err := m.Validate(); err == nil {
				t.Fatal("invalid product paint/source accepted")
			}
		})
	}
}
