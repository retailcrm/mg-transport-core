package internal

import "regexp"

// CredentialsTransport set of API methods for transport registration
var (
	CredentialsTransport = []string{
		"/api/integration-modules/{code}",
		"/api/integration-modules/{code}/edit",
	}
	MarkdownSymbols = []string{"*", "_", "`", "["}
	RegCommandName  = regexp.MustCompile(`^https://?[\da-z.-]+\.(retailcrm\.(ru|pro|es)|ecomlogic\.com|simlachat\.(com|ru))/?$`)
	SlashRegex      = regexp.MustCompile(`/+$`)
)
