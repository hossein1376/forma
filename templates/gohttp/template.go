package gohttp

import (
	"fmt"
	"regexp"
	"strconv"

	"github.com/hossein1376/forma"
	"github.com/hossein1376/forma/templates/internal/templateutil"
)

type Input struct {
	Name       string
	ModulePath string
	Port       int
	Docker     bool
	GitHubCI   bool
}

type Template struct{}

func (Template) Metadata() forma.Metadata {
	return forma.Metadata{
		Name:        "go-http-service",
		Description: "A small Go HTTP service with a health endpoint",
	}
}

var serviceNamePattern = regexp.MustCompile(
	`^[A-Za-z][A-Za-z0-9_-]*$`,
)

func (Template) Build(input Input) (forma.Component, error) {
	if !serviceNamePattern.MatchString(input.Name) {
		return nil, fmt.Errorf("name must match %s", serviceNamePattern.String())
	}
	if err := templateutil.ValidateModulePath(input.ModulePath); err != nil {
		return nil, err
	}
	if input.Port < 1 || input.Port > 65535 {
		return nil, fmt.Errorf("port must be between 1 and 65535")
	}

	port := strconv.Itoa(input.Port)
	return forma.Group(
		forma.File(
			"go.mod", templateutil.GoMod(input.ModulePath),
		),
		forma.Dir(
			"cmd",
			forma.Dir(
				input.Name,
				forma.File(
					"main.go",
					forma.Text(
						mainTemplate,
						forma.Values{"module": input.ModulePath, "port": port},
					),
				),
			),
		),
		forma.Dir(
			"internal",
			forma.Dir(
				"http",
				forma.File(
					"server.go", forma.Bytes([]byte(serverTemplate)),
				),
				forma.File(
					"server_test.go", forma.Bytes([]byte(serverTestTemplate)),
				),
			),
		),
		forma.File(
			"README.md",
			forma.Text(
				readmeTemplate, forma.Values{"name": input.Name, "port": port},
			),
		),
		forma.When(
			input.Docker,
			forma.File(
				"Dockerfile",
				forma.Text(
					dockerfileTemplate,
					forma.Values{
						"name":       input.Name,
						"port":       port,
						"go_version": templateutil.GoVersion,
					},
				),
			),
		),
		forma.When(
			input.GitHubCI,
			forma.Dir(
				".github",
				forma.Dir(
					"workflows", forma.File("test.yml", templateutil.Workflow()),
				),
			),
		),
	), nil
}

const mainTemplate = `package main

import (
	"errors"
	"log"
	"net/http"

	httpserver "${module}/internal/http"
)

func main() {
	server := httpserver.New(":${port}")
	log.Printf("listening on %s", server.Addr)
	if err := server.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
		log.Fatal(err)
	}
}
`

const serverTemplate = `package httpserver

import (
	"net/http"
	"time"
)

func New(address string) *http.Server {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", health)
	return &http.Server{
		Addr:              address,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}
}

func health(writer http.ResponseWriter, _ *http.Request) {
	writer.Header().Set("Content-Type", "text/plain; charset=utf-8")
	writer.WriteHeader(http.StatusOK)
	_, _ = writer.Write([]byte("ok\n"))
}
`

const serverTestTemplate = `package httpserver

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHealth(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/health", nil)
	response := httptest.NewRecorder()

	New(":0").Handler.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusOK)
	}
}
`

const readmeTemplate = `# ${name}

Run the service:

` + "```sh" + `
go run ./cmd/${name}
` + "```" + `

The health endpoint is available at ` + "`GET /health`" + ` on port ${port}.
`

const dockerfileTemplate = `FROM golang:${go_version} AS build
WORKDIR /src
COPY . .
RUN CGO_ENABLED=0 go build -o /service ./cmd/${name}

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /service /service
EXPOSE ${port}
ENTRYPOINT ["/service"]
`
