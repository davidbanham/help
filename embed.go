package help

import (
	"embed"
	_ "embed"
)

//go:embed assets/css/main.css
var cssFile []byte

//go:embed views/*
var views embed.FS
