# gsi-cop-infra
Code that powers the GSI CoP 

### Local development

Export ENVs:

```shell
export PROJECT_ID=partner-ecosystem-exchange
export GOOGLE_CLIENT_ID=...
export GOOGLE_CLIENT_SECRET=..
```

Run the service:

```shell
cd cmd/httpd

go run main.go
```

### Setup & Deployment

#### Google Cloud Platform prerequisites 

* OAuth 2.0 Client ID, with 'Authorized redirect URIs', 
* GAE Service Account, for local & GitHub workflow deployment
* APIs: App Engine, App Engine Admin API
* OAuth Consent Screen

#### GitHub Actions

Set the following 'Action secrets':

* GCP_SA_KEY
* GOOGLE_CLIENT_ID
* GOOGLE_CLIENT_SECRET
* APP_SECRET

Set the following 'Action variables':

* PROJECT_ID
* REGION
* BASE_URL

### Reference

* https://echo.labstack.com/docs

* https://github.com/labstack/echo

* https://pkg.go.dev/github.com/labstack/echo

* https://github.com/markbates/goth
