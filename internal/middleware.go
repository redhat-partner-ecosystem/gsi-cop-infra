package internal

import (
	"errors"
	"fmt"
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

		// Filesystem provides access to the static content.
		// Optional. Defaults to http.Dir(config.Root)
		Filesystem http.FileSystem `yaml:"-"`
	}
)

// Static returns a Static middleware to serves static content from the provided root directory.
func Static(root string) echo.MiddlewareFunc {
	config := StaticConfig{
		Index: INDEX_HTML,
		Root:  root,
	}

	return staticWithConfig(config)
}

func staticWithConfig(config StaticConfig) echo.MiddlewareFunc {
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

			if !IsAuthenticated(c) {
				if !isAllowed(c) {
					return c.Redirect(http.StatusTemporaryRedirect, fmt.Sprintf("%s%s", baseUrl, LoginUrl))
					//return echo.NewHTTPError(http.StatusUnauthorized, "Please provide valid credentials")
				}
				// let it pass, assuming there is a handler that will take care the request
			}

			p, err := url.PathUnescape(c.Request().URL.Path)
			if err != nil {
				return
			}

			name := path.Join(config.Root, path.Clean("/"+p)) // "/"+ for security

			file, err := config.Filesystem.Open(name)
			if err != nil {
				if !isIgnorableOpenFileError(err) {
					return err
				}

				// file with that path did not exist, so we continue down in middleware/handler chain, hoping that we end up in
				// handler that is meant to handle this request
				if err = next(c); err == nil {
					return nil
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

// isAllowed whitelists url paths that do not need authentication,
// e.g. all paths used during the user authentication.
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
