package server

import (
	"net/http"

	"fineuploader/dist"
)

const (
	// Dir is prefix of the assets in the bindata
	Dir = "../ui/"
	// Default is the default item to load if 404
	Default = "../ui/index.html"
	// DebugDir is the prefix of the assets in development mode
	DebugDir = "ui/"
	// DebugDefault is the default item to load if 404
	DebugDefault = "ui/index.html"
	// DefaultContentType is the content-type to return for the Default file
	DefaultContentType = "text/html; charset=utf-8"
)



// AssetsOpts configures the asset middleware
type AssetsOpts struct {
	// Develop when true serves assets from ui/build directory directly; false will use internal bindata.
	Develop bool
}

// Assets creates a middleware that will serve a single page app.
func Assets(opts AssetsOpts) http.Handler {
	var assets dist.Assets
	if opts.Develop {
		assets = &dist.DebugAssets{
			Dir:     DebugDir,
			Default: DebugDefault,
		}
	} else {
		assets = &dist.BindataAssets{
			Prefix:             Dir,
			Default:            Default,
			DefaultContentType: DefaultContentType,
		}
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assets.Handler().ServeHTTP(w, r)
	})
}

func AssetsFS(opts AssetsOpts) http.FileSystem {
	var assets dist.Assets
	if opts.Develop {
		assets = &dist.DebugAssets{
			Dir:     DebugDir,
			Default: DebugDefault,
		}
	} else {
		assets = &dist.BindataAssets{
			Prefix:             Dir,
			Default:            Default,
			DefaultContentType: DefaultContentType,
		}
	}

	return assets.FileSystem()
}
