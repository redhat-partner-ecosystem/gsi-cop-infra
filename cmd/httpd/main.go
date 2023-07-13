package main

import (
	"log"

	"github.com/gorilla/sessions"

	"github.com/labstack/echo-contrib/session"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"

	"github.com/txsvc/stdlib/v2"

	"github.com/redhat-partner-ecosystem/gsi-cop-infra/internal"
)

func setup() *echo.Echo {
	// create a new router instance
	e := echo.New()

	// some basic config
	e.Use(middleware.Recover())
	e.Use(middleware.CORSWithConfig(middleware.DefaultCORSConfig))

	// session support
	e.Use(session.Middleware(sessions.NewCookieStore([]byte(stdlib.GetString("APP_SECRET", "supersecret")))))

	// setup the OAuth2 provider
	e.GET(internal.LoginUrl, internal.Login)
	e.GET(internal.LogoutUrl, internal.Logout)
	e.GET(internal.CallbackUrl, internal.Callback)

	// the static part of the site comes from here
	e.Use(internal.Static(stdlib.GetString("CONTENT_ROOT", "../../../_site")))

	return e
}

func shutdown(*echo.Echo) {
	// TODO: implement your own stuff here
}

func init() {
	// only needed if deployed to GCP
	if !stdlib.Exists("PROJECT_ID") {
		log.Fatal("missing environment variable 'PROJECT_ID'")
	}
}

func main() {
	service, err := internal.NewHttp(setup, shutdown, nil)
	if err != nil {
		log.Fatal(err)
	}

	service.StartBlocking()
}
