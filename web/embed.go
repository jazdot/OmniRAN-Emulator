package web

import "embed"

// Assets contains the embedded React build files.
//go:embed dist/*
var Assets embed.FS
