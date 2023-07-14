package internal

import (
	"bytes"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/gorilla/sessions"
	"github.com/labstack/echo/v4"

	"github.com/txsvc/stdlib/v2"
)

const (
	maxAge      = 86400 * 7 // 1 week
	sessionName = "_psession"
)

var (
	store *sessions.CookieStore
)

func init() {
	isProd := stdlib.GetString("APP_ENV", "development") == "production"

	// initialize the session store
	store = sessions.NewCookieStore([]byte(stdlib.GetString("APP_SECRET", "supersecret")))
	store.MaxAge(maxAge)
	store.Options.Path = "/"
	store.Options.HttpOnly = true // HttpOnly should always be enabled
	store.Options.Secure = isProd
}

// GetFromSession retrieves a previously-stored value from the session.
// If no value has previously been stored at the specified key, it will return an error.
func GetFromSession(c echo.Context, key string) (string, error) {
	session, _ := store.Get(c.Request(), sessionName)
	value, err := getSessionValue(session, key)
	if err != nil {
		return "", errors.New("could not find a matching session for this request")
	}

	return value, nil
}

// StoreInSession stores a specified key/value pair in the session.
func StoreInSession(c echo.Context, key string, value string) error {
	session, _ := store.New(c.Request(), sessionName)

	if err := updateSessionValue(session, key, value); err != nil {
		return err
	}

	return session.Save(c.Request(), c.Response())
}

func getSessionValue(session *sessions.Session, key string) (string, error) {
	value := session.Values[key]
	if value == nil {
		return "", fmt.Errorf("could not find a matching session for this request")
	}

	rdata := strings.NewReader(value.(string))
	r, err := gzip.NewReader(rdata)
	if err != nil {
		return "", err
	}
	s, err := io.ReadAll(r)
	if err != nil {
		return "", err
	}

	return string(s), nil
}

func updateSessionValue(session *sessions.Session, key, value string) error {
	var b bytes.Buffer
	gz := gzip.NewWriter(&b)
	if _, err := gz.Write([]byte(value)); err != nil {
		return err
	}
	if err := gz.Flush(); err != nil {
		return err
	}
	if err := gz.Close(); err != nil {
		return err
	}

	session.Values[key] = b.String()
	return nil
}
