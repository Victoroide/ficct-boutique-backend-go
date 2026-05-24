// Package staticassets serves seeded product placeholders.
//
// Real production deployments store catalog photography in MinIO via the
// Express documents service. The seed data, however, must not depend on an
// admin manually uploading files for the demo to look complete. We therefore
// generate small SVG placeholders (one per seeded SKU) at request time and
// serve them at /static/products/<sku>.svg. The image is deterministic, branded
// and intentionally low-fi so it is obvious in screenshots that this is a
// placeholder, not a real photo.
package staticassets

import (
	"fmt"
	"net/http"
	"strings"
)

var placeholderColors = map[string][2]string{
	"BLZ-001": {"#fef3c7", "#92400e"}, // amber for blusas
	"PNT-001": {"#1c1917", "#fafaf9"}, // ink for pantalones
	"VST-001": {"#fce7f3", "#9d174d"}, // pink for vestidos
	"FLD-001": {"#fed7aa", "#9a3412"}, // accent for faldas
}

// Handler returns an http.Handler that serves /static/products/<sku>.svg.
// Unknown SKUs get a neutral default placeholder.
func Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sku := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/static/products/"), ".svg")
		bg, fg := lookupColors(sku)
		w.Header().Set("Content-Type", "image/svg+xml")
		w.Header().Set("Cache-Control", "public, max-age=86400")
		fmt.Fprintf(w, `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 600 600">`+
			`<rect width="600" height="600" fill="%s"/>`+
			`<text x="300" y="280" text-anchor="middle" font-family="DM Serif Display, Georgia, serif" font-size="48" fill="%s">FICCT</text>`+
			`<text x="300" y="340" text-anchor="middle" font-family="Inter, sans-serif" font-size="22" fill="%s" letter-spacing="6">%s</text>`+
			`<circle cx="300" cy="440" r="36" fill="%s" opacity="0.35"/>`+
			`</svg>`, bg, fg, fg, sku, fg)
	})
}

func lookupColors(sku string) (string, string) {
	if c, ok := placeholderColors[sku]; ok {
		return c[0], c[1]
	}
	return "#f5f5f4", "#44403c"
}
