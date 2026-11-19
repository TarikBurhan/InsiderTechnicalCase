package handlers

import (
	"net/http"

	"notifier/panel/docs"
)

const swaggerUIPage = `<!doctype html>
<html>
<head>
  <title>Notification Panel API</title>
  <meta charset="utf-8"/>
  <link rel="stylesheet" href="https://cdn.jsdelivr.net/npm/swagger-ui-dist@5.17.14/swagger-ui.css">
</head>
<body>
  <div id="swagger-ui"></div>
  <script src="https://cdn.jsdelivr.net/npm/swagger-ui-dist@5.17.14/swagger-ui-bundle.js"></script>
  <script>window.ui = SwaggerUIBundle({ url: '/swagger/openapi.yaml', dom_id: '#swagger-ui' });</script>
</body>
</html>`

// SwaggerUI serves the Swagger UI page.
func SwaggerUI(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write([]byte(swaggerUIPage))
}

// OpenAPISpec serves the embedded OpenAPI specification.
func OpenAPISpec(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/yaml")
	_, _ = w.Write(docs.OpenAPI)
}
