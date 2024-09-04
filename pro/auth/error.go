package auth

import (
	"fmt"
	"net/http"

	"github.com/gravitl/netmaker/servercfg"
)

var htmlBaseTemplate = `<!DOCTYPE html>
<html lang="en">

<head>
	<meta charset="UTF-8">
	<meta name="viewport" content="width=device-width, initial-scale=1.0, user-scalable=yes">
	<meta http-equiv="X-UA-Compatible" content="ie=edge">
	<title>Netmaker :: SSO</title>
	<script type="text/javascript">
	function redirect()
    {
    	window.location.href="` + fmt.Sprintf("https://dashboard.%s/login", servercfg.GetNmBaseDomain()) + `";
    }
	</script>
	<style>
		html,
		body {
			margin: 0px;
			padding: 0px;
		}

		body {
			height: 100vh;
			overflow: hidden;
			display: flex;
			flex-flow: column nowrap;
			justify-content: center;
			align-items: center;
		}

		#logo {
			width: 150px;
		}

		h3 {
			margin-bottom: 3rem;
			color: rgb(25, 135, 84);
			font-size: xx-large;
		}

		h4 {
			margin-bottom: 0px;
		}

		p {
			margin-top: 0px;
			margin-bottom: 0px;
		}
		.back-to-login-btn {
			background: #5E5DF0;
			border-radius: 999px;
			box-shadow: #5E5DF0 0 10px 20px -10px;
			box-sizing: border-box;
			color: #FFFFFF;
			cursor: pointer;
			font-family: Inter,Helvetica,"Apple Color Emoji","Segoe UI Emoji",NotoColorEmoji,"Noto Color Emoji","Segoe UI Symbol","Android Emoji",EmojiSymbols,-apple-system,system-ui,"Segoe UI",Roboto,"Helvetica Neue","Noto Sans",sans-serif;
			font-size: 16px;
			font-weight: 700;
			line-height: 24px;
			opacity: 1;
			outline: 0 solid transparent;
			padding: 8px 18px;
			user-select: none;
			-webkit-user-select: none;
			touch-action: manipulation;
			width: fit-content;
			word-break: break-word;
			border: 0;
			margin: 20px;
		  }
	</style>
</head>

<body>
	<img
		src="https://raw.githubusercontent.com/gravitl/netmaker-docs/master/images/netmaker-github/netmaker-teal.png"
		alt="netmaker logo"
		id="logo"
	>
	%s
	<button class="back-to-login-btn" onClick="redirect()" role="button">Back To Login</button>
	
</body>
</html>`

var oauthNotConfigured = fmt.Sprintf(htmlBaseTemplate, `<h2>Your Netmaker server does not have OAuth configured.</h2>
<p>Please visit the docs <a href="https://docs.netmaker.org/oauth.html" target="_blank" rel="noopener">here</a> to learn how to.</p>`)

var oauthStateInvalid = fmt.Sprintf(htmlBaseTemplate, `<h2>Invalid OAuth Session. Please re-try again.</h2>`)

var userNotAllowed = fmt.Sprintf(htmlBaseTemplate, `<h2>Your account does not have access to the dashboard. Please contact your administrator for more information about your account.</h2>
<p>Non-Admins can access the netmaker networks using <a href="https://docs.netmaker.io/pro/rac.html" target="_blank" rel="noopener">RemoteAccessClient.</a></p>`)

var userFirstTimeSignUp = fmt.Sprintf(htmlBaseTemplate, `<h2>Thank you for signing up. Please contact your administrator for access.</h2>`)

var userSignUpApprovalPending = fmt.Sprintf(htmlBaseTemplate, `<h2>Your account is yet to be approved. Please contact your administrator for access.</h2>`)

var userNotFound = fmt.Sprintf(htmlBaseTemplate, `<h2>User Not Found.</h2>`)

var somethingwentwrong = fmt.Sprintf(htmlBaseTemplate, `<h2>Something went wrong. Contact Admin.</h2>`)

var notallowedtosignup = fmt.Sprintf(htmlBaseTemplate, `<h2>Your email is not allowed. Please contact your administrator.</h2>`)

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
func handleFirstTimeOauthUserSignUp(response http.ResponseWriter) {
	response.Header().Set("Content-Type", "text/html; charset=utf-8")
	response.WriteHeader(http.StatusForbidden)
	response.Write([]byte(userFirstTimeSignUp))
}

func handleOauthUserSignUpApprovalPending(response http.ResponseWriter) {
	response.Header().Set("Content-Type", "text/html; charset=utf-8")
	response.WriteHeader(http.StatusForbidden)
	response.Write([]byte(userSignUpApprovalPending))
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

func handleOauthNotValid(response http.ResponseWriter) {
	response.Header().Set("Content-Type", "text/html; charset=utf-8")
	response.WriteHeader(http.StatusBadRequest)
	response.Write([]byte(oauthStateInvalid))
}

func handleSomethingWentWrong(response http.ResponseWriter) {
	response.Header().Set("Content-Type", "text/html; charset=utf-8")
	response.WriteHeader(http.StatusInternalServerError)
	response.Write([]byte(somethingwentwrong))
}
