package auth

var okta_functions = map[string]interface{}{
	init_provider:   initOIDC,
	get_user_info:   getOIDCUserInfo,
	handle_callback: handleOIDCCallback,
	handle_login:    handleOIDCLogin,
	verify_user:     verifyOIDCUser,
}
