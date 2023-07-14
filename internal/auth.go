package internal

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"

	"github.com/labstack/echo/v4"
	"github.com/markbates/goth"
	provider "github.com/markbates/goth/providers/google"

	"github.com/txsvc/stdlib/v2"
)

const (
	LoginUrl    = "/_p/login"
	LogoutUrl   = "/_p/logout"
	CallbackUrl = "/_p/callback"

	AuthNamespace = "/_p/" // allow requests starting with this prefix

	UserID          = "uid"
	UserAccessToken = "uatk"
)

var baseUrl = stdlib.GetString("BASE_URL", "http://localhost:8080")

func init() {
	// initilaize the provider
	goth.UseProviders(provider.New(
		stdlib.GetString("GOOGLE_CLIENT_ID", ""), stdlib.GetString("GOOGLE_CLIENT_SECRET", ""),
		fmt.Sprintf("%s%s", baseUrl, CallbackUrl),
		"https://www.googleapis.com/auth/userinfo.email",
		"https://www.googleapis.com/auth/userinfo.profile"),
	)
}

func IsAuthenticated(c echo.Context) bool {
	if _, err := GetFromSession(c, UserID); err == nil {
		return true
	}

	return false
}

func Authenticate(c echo.Context, user goth.User) error {
	if err := StoreInSession(c, UserID, user.Email); err != nil {
		return err
	}

	return nil
}

func Login(c echo.Context) error {
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

	if err := StoreInSession(c, providerName, sess.Marshal()); err != nil {
		return err
	}

	return c.Redirect(http.StatusTemporaryRedirect, url)
}

func Logout(c echo.Context) error {
	req := c.Request()
	session, err := store.Get(req, sessionName)
	if err != nil {
		return err
	}

	session.Options.MaxAge = -1
	session.Values = make(map[interface{}]interface{})

	if err := session.Save(req, c.Response()); err != nil {
		return errors.New("could not delete session ")
	}

	return c.Redirect(http.StatusTemporaryRedirect, baseUrl) // FIXME make this configurable
}

func Callback(c echo.Context) error {
	providerName, _ := GetProviderName(c)

	provider, err := goth.GetProvider(providerName)
	if err != nil {
		return err
	}

	value, err := GetFromSession(c, providerName)
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

	if _, err := provider.FetchUser(sess); err == nil {
		// user can be found within the existing session data
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
	if _, err := sess.Authorize(provider, params); err != nil {
		return err
	}

	if err := StoreInSession(c, providerName, sess.Marshal()); err != nil {
		return err
	}

	user, err := provider.FetchUser(sess)
	if err != nil {
		return err
	}

	if err = Authenticate(c, user); err != nil {
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

// GetProviderName is a function used to get the name of a provider
// for a given request. By default, this provider is fetched from
// the URL query string.
func GetProviderName(c echo.Context) (string, error) {
	return "google", nil // FIXME: only Google is supported for now

	/*
		if p := c.Param("provider"); p != "" {
			return p, nil
		}

		return gothic.GetProviderName(c.Request())
	*/
}

// SetState sets the state string associated with the given request.
// If no state string is associated with the request, one will be generated.
// This state is sent to the provider and can be retrieved during the callback.
func SetState(c echo.Context) string {
	state := c.Request().URL.Query().Get("state")
	if len(state) > 0 {
		return state
	}

	// If a state query param is not passed in, generate a random
	// base64-encoded nonce so that the state on the auth URL
	// is unguessable, preventing CSRF attacks, as described in
	//
	// https://auth0.com/docs/protocols/oauth2/oauth-state#keep-reading
	nonceBytes := make([]byte, 64)
	_, err := io.ReadFull(rand.Reader, nonceBytes)
	if err != nil {
		panic("source of randomness unavailable: " + err.Error())
	}

	return base64.URLEncoding.EncodeToString(nonceBytes)
}

// GetState gets the state returned by the provider during the callback.
// This is used to prevent CSRF attacks, see
// http://tools.ietf.org/html/rfc6749#section-10.12
func GetState(c echo.Context) string {
	req := c.Request()
	params := req.URL.Query()

	if params.Encode() == "" && req.Method == http.MethodPost {
		return req.FormValue("state")
	}

	return params.Get("state")
}
