package auth

import (
	"net/http"

	"github.com/gravitl/netmaker/logger"
	"github.com/gravitl/netmaker/schema"
)

// ValidateEmail - handles a user clicking the email-validation link sent to
// them on MSP deployments: marks their email validated and consumes the
// invite.
// @Summary     Validate a user's email
// @Router      /api/v1/users/validate-email [get]
// @Tags        Users
// @Produce     html
// @Param       invite_code query string true "Validation invite code"
// @Param       email query string true "User email"
func ValidateEmail(w http.ResponseWriter, r *http.Request) {
	code := r.URL.Query().Get("invite_code")
	userEmail := r.URL.Query().Get("email")

	invite := &schema.UserInvite{
		InviteCode: code,
		Email:      userEmail,
	}
	err := invite.GetValidationInvite(r.Context())
	if err != nil {
		logger.Log(0, "failed to fetch email validation invite: ", err.Error())
		handleInvalidValidationLink(w)
		return
	}

	user := &schema.User{Username: userEmail}
	err = user.Get(r.Context())
	if err != nil {
		logger.Log(0, "failed to fetch user for email validation: ", err.Error())
		handleInvalidValidationLink(w)
		return
	}

	user.EmailValidated = true
	err = user.Update(r.Context())
	if err != nil {
		logger.Log(0, "failed to mark email validated: ", err.Error())
		handleSomethingWentWrong(w)
		return
	}

	_ = invite.Delete(r.Context())

	handleEmailValidated(w)
}
