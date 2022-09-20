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
	  <meta name="viewport" content="width=device-width, initial-scale=1.0">
	  <meta http-equiv="X-UA-Compatible" content="ie=edge">
	  <link rel="stylesheet" href="https://cdn.jsdelivr.net/npm/bootstrap@4.3.1/dist/css/bootstrap.min.css"
		integrity="sha384-ggOyR0iXCbMQv3Xipma34MD+dH/1fQ784/j6cY/iJTQUOhcWr7x9JvoRxT2MZw1T" crossorigin="anonymous">
	  <title>Netmaker</title>
	</head>
	<style>
	  .text-responsive {
		font-size: calc(100% + 1vw + 1vh);
	  }
	</style>
	<body>
	  <div class="container">
		<div class="row justify-content-center mt-5 p-5 align-items-center text-center">
		  <a href="https://netmaker.io">
			<img src="https://raw.githubusercontent.com/gravitl/netmaker/master/img/netmaker-teal.png" alt="Netmaker"
			  width="75%" height="25%" class="img-fluid">
		  </a>
		</div>
		<div class="row justify-content-center mt-5 p-3 text-center">
		  <div class="col">
			<h2 class="text-responsive">{{.User}} has been successfully {{.Verb}}</h2>
			<br />
			<h2 class="text-responsive">You may now close this window.</h2>
		  </div>
		</div>
	  </div>
	</body>
	</html>`),
)

var ssoErrCallbackTemplate = template.Must(
	template.New("ssocallback").Parse(`<!DOCTYPE html>
	<html lang="en">
	<head>
	  <meta charset="UTF-8">
	  <meta name="viewport" content="width=device-width, initial-scale=1.0">
	  <meta http-equiv="X-UA-Compatible" content="ie=edge">
	  <link rel="stylesheet" href="https://cdn.jsdelivr.net/npm/bootstrap@4.3.1/dist/css/bootstrap.min.css"
		integrity="sha384-ggOyR0iXCbMQv3Xipma34MD+dH/1fQ784/j6cY/iJTQUOhcWr7x9JvoRxT2MZw1T" crossorigin="anonymous">
	  <title>Netmaker</title>
	</head>
	<style>
	  .text-responsive {
		font-size: calc(100% + 1vw + 1vh);
		color: red;
	  }
	</style>
	<body>
	  <div class="container">
		<div class="row justify-content-center mt-5 p-5 align-items-center text-center">
		  <a href="https://netmaker.io">
			<img src="https://raw.githubusercontent.com/gravitl/netmaker/master/img/netmaker-teal.png" alt="Netmaker"
			  width="75%" height="25%" class="img-fluid">
		  </a>
		</div>
		<div class="row justify-content-center mt-5 p-3 text-center">
		  <div class="col">
			<h2 class="text-responsive">{{.User}} unable to join network: {{.Verb}}</h2>
			<br />
			<h2 class="text-responsive">If you feel this is a mistake, please contact your network administrator.</h2>
		  </div>
		</div>
	  </div>
	</body>
	</html>`),
)
