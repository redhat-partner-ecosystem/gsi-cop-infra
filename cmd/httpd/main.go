package main

import (
	"log"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"

	"github.com/txsvc/stdlib/v2"

	"github.com/redhat-partner-ecosystem/gsi-cop-infra/internal/httpserver"
)

func setup() *echo.Echo {
	// create a new router instance
	e := echo.New()

	// add and configure the middleware(s)
	e.Use(middleware.Recover())
	e.Use(middleware.CORSWithConfig(middleware.DefaultCORSConfig))

	// the static part of the site comes from here
	e.Static("/", stdlib.GetString("CONTENT_ROOT", "../../_site"))

	// TODO add your own endpoints here
	//e.GET("/", api.DefaultEndpoint)

	/* debug
	files, err := os.ReadDir(".")
	if err != nil {
		log.Fatal(err)
	}

	for _, f := range files {
		fmt.Println(f.Name())
	}
	*/

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
	service, err := httpserver.New(setup, shutdown, nil)
	if err != nil {
		log.Fatal(err)
	}

	service.StartBlocking()
}
