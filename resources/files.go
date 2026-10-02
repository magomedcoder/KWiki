package resources

import "embed"

//go:embed templates
//go:embed locales
//go:embed css/tailwindcss.css
//go:embed js/main.js
var FS embed.FS
