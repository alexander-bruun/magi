package embedded

import "io/fs"

// Package-level filesystem variables for embedded assets.
// Set once at startup via Init() and read by consumer packages.
var (
	Views      fs.FS
	Assets     fs.FS
	Migrations fs.FS
)

// Init sets the embedded filesystems. Must be called before any
// consumer (models, handlers, template engine) accesses them.
func Init(views, assets, migrations fs.FS) {
	Views = views
	Assets = assets
	Migrations = migrations
}
