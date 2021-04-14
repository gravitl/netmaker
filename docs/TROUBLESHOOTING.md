# Netmaker Troubleshooting Help

## Client (netclient)

### Client fails to install

### Cannot run install script

### Issue with accesstoken created by UI


## Server

## UI

## MongoDB



## Incorrect backend detected. Please specify correct URL and refresh. Given: http://localhost:8081
Solution: Front end expects a reachable address for the backend. Localhost is default. Check if server is up. If server is up, make sure you've got the right endpoint (endpoint of server. Will not be 'localhost' unless doing local testing). If server is up and endpoint is correct, check for port blockings.
