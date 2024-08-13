API calls are primarily authenticated using a user authentication token. This token should be included in the header as follows:

-H "Authorization: Bearer <YOUR_AUTH_TOKEN>"

To obtain YOUR_AUTH_TOKEN:
Call the api/users/adm/authenticate endpoint (see documentation below for details).

Note: While a MasterKey exists (configurable via env var or config file), it should be considered a backup option, used only when server access is lost. By default, this key is "secret key," but it's crucial to change this and keep it secure in your instance.

For more information on configuration and security best practices, refer to the [Netmaker documentation](https://docs.netmaker.org/index.html).
