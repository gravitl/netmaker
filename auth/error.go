package auth

import "net/http"

// == define error HTML here ==
const oauthNotConfigured = `<!DOCTYPE html><html>
<body>
<h3>Your Netmaker server does not have OAuth configured.</h3>
<p>Please visit the docs <a href="https://docs.netmaker.org/oauth.html" target="_blank" rel="noopener">here</a> to learn how to.</p>
</body>
</html>`

// handleOauthNotConfigured - returns an appropriate html page when oauth is not configured on netmaker server but an oauth login was attempted
func handleOauthNotConfigured(response http.ResponseWriter) {
	response.Header().Set("Content-Type", "text/html; charset=utf-8")
	response.WriteHeader(http.StatusInternalServerError)
	response.Write([]byte(oauthNotConfigured))
}
