//Package classification Netmaker
//
// API Usage
//
// Most actions that can be performed via API can be performed via UI. We recommend managing your networks using the official netmaker-ui project. However, Netmaker can also be run without the UI, and all functions can be achieved via API calls. If your use case requires using Netmaker without the UI or you need to do some troubleshooting/advanced configuration, using the API directly may help.
//
//
// Authentication
//
// API calls must be authenticated via a header of the format -H “Authorization: Bearer <YOUR_SECRET_KEY>” There are two methods to obtain YOUR_SECRET_KEY: 1. Using the masterkey. By default, this value is “secret key,” but you should change this on your instance and keep it secure. This value can be set via env var at startup or in a config file (config/environments/< env >.yaml). See the [Netmaker](https://docs.netmaker.org/index.html) documentation for more details. 2. Using a JWT received for a node. This can be retrieved by calling the /api/nodes/<network>/authenticate endpoint, as documented below.
//
//     Schemes: https
//     BasePath: /
//     Version: 0.15.1
//     Host: netmaker.io
//
//     Consumes:
//     - application/json
//
//     Produces:
//     - application/json
//
//     Security:
//     - oauth
//
// swagger:meta
package controller
