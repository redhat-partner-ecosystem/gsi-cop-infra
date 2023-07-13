package internal

import (
	"errors"
	"net/http"
	"net/url"
	"os"
	"path"
	"strings"

	"github.com/labstack/echo/v4"
)

const (
	INDEX_HTML = "index.html"
)

type (
	// StaticConfig defines the config for Static middleware.
	StaticConfig struct {
		// Root directory from where the static content is served.
		// Required.
		Root string `yaml:"root"`

		// Index file for serving a directory.
		// Optional. Default value "index.html".
		Index string `yaml:"index"`

		// Enable HTML5 mode by forwarding all not-found requests to root so that
		// SPA (single-page application) can handle the routing.
		// Optional. Default value false.
		HTML5 bool `yaml:"html5"`

		// FIXME remove this
		// Enable directory browsing.
		// Optional. Default value false.
		//Browse bool `yaml:"browse"`

		// Enable ignoring of the base of the URL path.
		// Example: when assigning a static middleware to a non root path group,
		// the filesystem path is not doubled
		// Optional. Default value false.
		IgnoreBase bool `yaml:"ignoreBase"`

		// Filesystem provides access to the static content.
		// Optional. Defaults to http.Dir(config.Root)
		Filesystem http.FileSystem `yaml:"-"`
	}
)

// Static returns a Static middleware to serves static content from the provided
// root directory.
func Static(root string) echo.MiddlewareFunc {
	c := StaticConfig{
		Index: INDEX_HTML,
	}
	c.Root = root

	return StaticWithConfig(c)
}

// StaticWithConfig returns a Static middleware with config.
// See `Static()`.
func StaticWithConfig(config StaticConfig) echo.MiddlewareFunc {
	// Defaults
	if config.Root == "" {
		config.Root = "." // For security we want to restrict to CWD.
	}
	if config.Index == "" {
		config.Index = INDEX_HTML
	}
	if config.Filesystem == nil {
		config.Filesystem = http.Dir(config.Root)
		config.Root = "."
	}

	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) (err error) {
			p := c.Request().URL.Path
			//fmt.Println(p)

			if !IsAuthenticated(c) {
				if !isAllowed(c) {
					return echo.NewHTTPError(http.StatusUnauthorized, "Please provide valid credentials")
				}
				// let it pass, assuming there is a handler that will take care of it
			}

			if strings.HasSuffix(c.Path(), "*") { // When serving from a group, e.g. `/static*`.
				p = c.Param("*")
			}
			p, err = url.PathUnescape(p)
			if err != nil {
				return
			}
			name := path.Join(config.Root, path.Clean("/"+p)) // "/"+ for security

			if config.IgnoreBase {
				routePath := path.Base(strings.TrimRight(c.Path(), "/*"))
				baseURLPath := path.Base(p)
				if baseURLPath == routePath {
					i := strings.LastIndex(name, routePath)
					name = name[:i] + strings.Replace(name[i:], routePath, "", 1)
				}
			}

			file, err := config.Filesystem.Open(name)
			if err != nil {
				if !isIgnorableOpenFileError(err) {
					return err
				}

				// file with that path did not exist, so we continue down in middleware/handler chain, hoping that we end up in
				// handler that is meant to handle this request
				if err = next(c); err == nil {
					return err
				}

				var he *echo.HTTPError
				if !(errors.As(err, &he) && config.HTML5 && he.Code == http.StatusNotFound) {
					return err
				}

				file, err = config.Filesystem.Open(path.Join(config.Root, config.Index))
				if err != nil {
					return err
				}
			}

			defer file.Close()

			info, err := file.Stat()
			if err != nil {
				return err
			}

			if info.IsDir() {
				index, err := config.Filesystem.Open(path.Join(name, config.Index))
				if err != nil {
					return next(c)
				}

				defer index.Close()

				info, err = index.Stat()
				if err != nil {
					return err
				}

				return serveFile(c, index, info)
			}

			return serveFile(c, file, info)
		}
	}
}

// isAllowed whitelists url paths that do not need authentication
func isAllowed(c echo.Context) bool {
	p := c.Request().URL.Path
	return strings.HasPrefix(p, AuthNamespace)
}

// We ignore these errors as there could be handler that matches request path.
func isIgnorableOpenFileError(err error) bool {
	return os.IsNotExist(err)
}

func serveFile(c echo.Context, file http.File, info os.FileInfo) error {
	http.ServeContent(c.Response(), c.Request(), info.Name(), info.ModTime(), file)
	return nil
}
