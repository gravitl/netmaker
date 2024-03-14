package auth

import "net/http"

// == define error HTML here ==
const oauthNotConfigured = `<!DOCTYPE html><html>
<body>
<h3>Your Netmaker server does not have OAuth configured.</h3>
<p>Please visit the docs <a href="https://docs.netmaker.org/oauth.html" target="_blank" rel="noopener">here</a> to learn how to.</p>
</body>
</html>`

const userNotAllowed = `<!DOCTYPE html><html>
<body>
<h3>Only Admins are allowed to access Dashboard.</h3>
<h3>Furthermore, Admin has to approve your identity to have access to netmaker networks</h3>
<p>Once your identity is approved, Non-Admins can access the netmaker networks using <a href="https://docs.netmaker.io/pro/rac.html" target="_blank" rel="noopener">RemoteAccessClient.</a></p>
</body>
</html>
`
const userNotFound = `<!DOCTYPE html><html>
<body>
<h3>User Not Found.</h3>
</body>
</html>`

const somethingwentwrong = `<!DOCTYPE html><html>
<body>
<h3>Something went wrong. Contact Admin</h3>
</body>
</html>`

const notallowedtosignup = `<!DOCTYPE html><html>
<body>
<h3>You are not allowed to SignUp.</h3>
</body>
</html>`

func handleOauthUserNotFound(response http.ResponseWriter) {
	response.Header().Set("Content-Type", "text/html; charset=utf-8")
	response.WriteHeader(http.StatusNotFound)
	response.Write([]byte(userNotFound))
}

func handleOauthUserNotAllowed(response http.ResponseWriter) {
	response.Header().Set("Content-Type", "text/html; charset=utf-8")
	response.WriteHeader(http.StatusForbidden)
	response.Write([]byte(userNotAllowed))
}

func handleOauthUserNotAllowedToSignUp(response http.ResponseWriter) {
	response.Header().Set("Content-Type", "text/html; charset=utf-8")
	response.WriteHeader(http.StatusForbidden)
	response.Write([]byte(notallowedtosignup))
}

// handleOauthNotConfigured - returns an appropriate html page when oauth is not configured on netmaker server but an oauth login was attempted
func handleOauthNotConfigured(response http.ResponseWriter) {
	response.Header().Set("Content-Type", "text/html; charset=utf-8")
	response.WriteHeader(http.StatusInternalServerError)
	response.Write([]byte(oauthNotConfigured))
}

func handleSomethingWentWrong(response http.ResponseWriter) {
	response.Header().Set("Content-Type", "text/html; charset=utf-8")
	response.WriteHeader(http.StatusInternalServerError)
	response.Write([]byte(somethingwentwrong))
}
