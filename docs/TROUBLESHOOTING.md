# Netmaker Troubleshooting Help

## Client (netclient)
	### Problem: netclient-install script not working
	### Problem: Hanging artifacts from previous install
	### Problem: Need to change access token settings


### Client fails to install

### Cannot run install script

### Issue with accesstoken created by UI


## Server
	### Server not added to default network
	### Global config not found



## MongoDB



## UI

### Incorrect backend detected. Please specify correct URL and refresh. Given: http://localhost:8081
Solution: Front end expects a reachable address for the backend. Localhost is default. Check if server is up. If server is up, make sure you've got the right endpoint (endpoint of server. Will not be 'localhost' unless doing local testing). If server is up and endpoint is correct, check for port blockings.
