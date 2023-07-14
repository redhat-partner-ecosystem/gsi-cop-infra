package internal

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path"
	"strconv"
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

			// check if the request is already authenticated

			if !IsAuthenticated(c) {
				if !isAllowed(c) {
					return c.Redirect(http.StatusTemporaryRedirect, fmt.Sprintf("%s%s", baseUrl, LoginUrl))
					//return echo.NewHTTPError(http.StatusUnauthorized, "Please provide valid credentials")
				}
				// let it pass, assuming there is a handler that will take care the request
			}

			// what is requested ?

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

// calculateEtag produces a strong etag by default, although, for
// efficiency reasons, it does not actually consume the contents
// of the file to make a hash of all the bytes. ¯\_(ツ)_/¯
// Prefix the etag with "W/" to convert it into a weak etag.
// See: https://tools.ietf.org/html/rfc7232#section-2.3
func calculateEtag(d os.FileInfo) string {
	mtime := d.ModTime().Unix()
	if mtime == 0 || mtime == 1 {
		return "" // not useful anyway; see issue #5548
	}
	t := strconv.FormatInt(mtime, 36)
	s := strconv.FormatInt(d.Size(), 36)
	return `"` + t + s + `"`
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
	req := c.Request()
	resp := c.Response()

	// at this point, we're serving a file; Go std lib supports only
	// GET and HEAD, which is sensible for a static file server - reject
	// any other methods (see issue #5166)
	if req.Method != http.MethodGet && req.Method != http.MethodHead {
		resp.Header().Add("Allow", "GET, HEAD")
		return echo.NewHTTPError(http.StatusMethodNotAllowed, nil)
	}

	// etag is usually unset, but if the user knows what they're doing, let them override it
	etag := resp.Header().Get("Etag")
	if etag == "" {
		etag = calculateEtag(info)
	}

	// set the Etag - note that a conditional If-None-Match request is handled
	// by http.ServeContent below, which checks against this Etag value
	if etag != "" {
		resp.Header().Set("Etag", etag)
	}

	http.ServeContent(c.Response(), c.Request(), info.Name(), info.ModTime(), file)
	return nil
}
