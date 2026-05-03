package conventions

// Well-known service names for OTEL service.name and Datadog "service" tag.
// Use these constants instead of literal strings to keep the service-name
// taxonomy consistent across infra config, dashboards, and code.
const (
	ServiceMCP        = "jinko-mcp"
	ServiceMCPBFF     = "jinko-mcp-bff"
	ServiceConnector  = "jinko-connector"
	ServiceCLI        = "jinko-cli"
	ServiceDevTools   = "jinko-dev-tools"
	ServiceAppWeb     = "jinko-app-web"
	ServiceBackoffice = "jinko-backoffice"
)
