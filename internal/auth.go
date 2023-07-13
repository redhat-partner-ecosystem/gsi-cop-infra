package internal

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"

	"github.com/gorilla/sessions"
	"github.com/labstack/echo/v4"
	"github.com/markbates/goth"
	"github.com/markbates/goth/gothic"

	provider "github.com/markbates/goth/providers/google"

	"github.com/txsvc/stdlib/v2"
)

const (
	LoginUrl      = "/_p/login"
	LogoutUrl     = "/_p/logout"
	CallbackUrl   = "/_p/callback"
	AuthNamespace = "/_p/"

	maxAge = 86400 * 30 // 30 days
)

var baseUrl = stdlib.GetString("BASE_URL", "http://localhost:8080")

func init() {
	isProd := stdlib.GetString("APP_ENV", "development") == "production"

	// initialize the session store
	store := sessions.NewCookieStore([]byte(stdlib.GetString("APP_SECRET", "supersecret")))
	store.MaxAge(maxAge)
	store.Options.Path = "/"
	store.Options.HttpOnly = true // HttpOnly should always be enabled
	store.Options.Secure = isProd
	gothic.Store = store

	// initilaize the provider
	goth.UseProviders(provider.New(
		stdlib.GetString("GOOGLE_CLIENT_ID", ""), stdlib.GetString("GOOGLE_CLIENT_SECRET", ""),
		fmt.Sprintf("%s%s", baseUrl, CallbackUrl),
		"https://www.googleapis.com/auth/userinfo.email"),
	)
}

func Login(c echo.Context) error {
	//fmt.Println("--> login")
	providerName, err := GetProviderName(c)
	if err != nil {
		return err
	}

	provider, err := goth.GetProvider(providerName)
	if err != nil {
		return err
	}

	sess, err := provider.BeginAuth(SetState(c))
	if err != nil {
		return err
	}

	url, err := sess.GetAuthURL()
	if err != nil {
		return err
	}

	err = StoreInSession(providerName, sess.Marshal(), c)
	if err != nil {
		return err
	}

	return c.Redirect(http.StatusTemporaryRedirect, url)
}

func Logout(c echo.Context) error {
	//fmt.Println("--> logout")
	gothic.Logout(c.Response(), c.Request())
	return c.Redirect(http.StatusTemporaryRedirect, baseUrl)
}

func Callback(c echo.Context) error {
	//fmt.Println("--> callback")
	providerName, _ := GetProviderName(c)

	provider, err := goth.GetProvider(providerName)
	if err != nil {
		return err
	}

	value, err := GetFromSession(providerName, c)
	if err != nil {
		return err
	}

	sess, err := provider.UnmarshalSession(value)
	if err != nil {
		return err
	}

	if err = validateState(c, sess); err != nil {
		return err
	}

	_, err = provider.FetchUser(sess)
	if err == nil {
		// user can be found with existing session data
		return nil
	}

	params := c.Request().URL.Query()
	if params.Encode() == "" && c.Request().Method == "POST" {
		err = c.Request().ParseForm()
		if err != nil {
			return err
		}
		params = c.Request().Form
	}

	// get new token and retry fetch
	_, err = sess.Authorize(provider, params)
	if err != nil {
		return err
	}

	err = StoreInSession(providerName, sess.Marshal(), c)

	if err != nil {
		return err
	}

	user, err := provider.FetchUser(sess)
	if err != nil {
		return err
	}

	if err = gothic.StoreInSession("email", user.Email, c.Request(), c.Response()); err != nil {
		return err
	}

	return c.Redirect(http.StatusTemporaryRedirect, baseUrl)
}

// validateState ensures that the state token param from the original
// AuthURL matches the one included in the current (callback) request.
func validateState(c echo.Context, sess goth.Session) error {
	rawAuthURL, err := sess.GetAuthURL()
	if err != nil {
		return err
	}

	authURL, err := url.Parse(rawAuthURL)
	if err != nil {
		return err
	}

	reqState := GetState(c)

	originalState := authURL.Query().Get("state")
	if originalState != "" && (originalState != reqState) {
		return errors.New("state token mismatch")
	}
	return nil
}

func IsAuthenticated(c echo.Context) bool {
	if _, err := gothic.GetFromSession("email", c.Request()); err == nil {
		return true
	}
	return false
}

// GetProviderName is a function used to get the name of a provider
// for a given request. By default, this provider is fetched from
// the URL query string. If you provide it in a different way,
// assign your own function to this variable that returns the provider
// name for your request.
func GetProviderName(c echo.Context) (string, error) {
	return "google", nil // only Google is supported for now
	/*
		if p := c.Param("provider"); p != "" {
			return p, nil
		}

		return gothic.GetProviderName(c.Request())
	*/
}

// SetState sets the state string associated with the given request.
// If no state string is associated with the request, one will be generated.
// This state is sent to the provider and can be retrieved during the
// callback.
func SetState(c echo.Context) string {
	return gothic.SetState(c.Request())
}

// GetState gets the state returned by the provider during the callback.
// This is used to prevent CSRF attacks, see http://tools.ietf.org/html/rfc6749#section-10.12
func GetState(c echo.Context) string {
	return gothic.GetState(c.Request())
}

// StoreInSession stores a specified key/value pair in the session.
func StoreInSession(key string, value string, c echo.Context) error {
	return gothic.StoreInSession(key, value, c.Request(), c.Response())
}

// GetFromSession retrieves a previously-stored value from the session.
// If no value has previously been stored at the specified key, it will return an error.
func GetFromSession(key string, c echo.Context) (string, error) {
	return gothic.GetFromSession(key, c.Request())
}
