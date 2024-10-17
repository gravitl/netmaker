package auth

import "html/template"

type ssoCallbackTemplateConfig struct {
	User string
	Verb string
}

var ssoCallbackTemplate = template.Must(
	template.New("ssocallback").Parse(`<!DOCTYPE html>
	<html lang="en">
	
	<head>
		<meta charset="UTF-8">
		<meta name="viewport" content="width=device-width, initial-scale=1.0, user-scalable=yes">
		<meta http-equiv="X-UA-Compatible" content="ie=edge">
		<title>Netmaker :: SSO Success</title>
	
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
		</style>
	</head>
	
	<body>
		<img
			src="https://raw.githubusercontent.com/gravitl/netmaker-docs/master/images/netmaker-github/netmaker-teal.png"
			alt="netmaker logo"
			id="logo"
		>
		<h3>Server SSO Success</h3>
		<h4>User {{.User}} has been successfully {{.Verb}}.</h4>
		<p>You can close this window now</p>
	</body>
	
	</html>`),
)

var ssoErrCallbackTemplate = template.Must(
	template.New("ssocallback").Parse(`<!DOCTYPE html>
	<html lang="en">
	
	<head>
		<meta charset="UTF-8">
		<meta name="viewport" content="width=device-width, initial-scale=1.0, user-scalable=yes">
		<meta http-equiv="X-UA-Compatible" content="ie=edge">
		<title>Netmaker :: SSO Error</title>
	
		<style>
			html, body {
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
				color:rgb(223, 71, 89);
				font-size: xx-large;
			}
			h4 {
				margin-top: 0rem;
			}
			p {
				margin-top: 3rem;
			}
		</style>
	</head>

	<body>
		<img
			src="https://raw.githubusercontent.com/gravitl/netmaker-docs/master/images/netmaker-github/netmaker-teal.png"
			alt="netmaker logo"
			id="logo"
		>
		<h3>Server SSO Error</h3>
		<h4>Error reason: {.Verb}</h4>
		<em>Your Netmaker server may not have SSO configured properly.</em>
		<em>
			Please visit the <a href="https://docs.netmaker.io/docs/server-installation/integrating-oauth" target="_blank" rel="noopener">docs</a> for more information.
		</em>
		<p>
			If you feel this is a mistake, please contact your network administrator.
		</p>
	</body>

	</html>`),
)
