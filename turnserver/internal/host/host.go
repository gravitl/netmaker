package host

import (
	"errors"
	"fmt"
	"log"

	"github.com/gin-gonic/gin"

	"github.com/gravitl/netmaker/logger"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/turnserver/internal/auth"
	errpkg "github.com/gravitl/netmaker/turnserver/internal/errors"
	"github.com/gravitl/netmaker/turnserver/internal/utils"
)

// Register - handles the host registration
func Register(c *gin.Context) {
	req := models.HostTurnRegister{}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.ReturnErrorResponse(c, errpkg.FormatError(err, errpkg.Internal))
		return
	}
	log.Printf("----> REG: %+v", req)
	auth.RegisterNewHostWithTurn(req.HostID, req.HostPassHash)
	utils.ReturnSuccessResponse(c,
		fmt.Sprintf("registered host (%s) successfully", req.HostID), nil)
}

// Remove - unregisters the host from turn server
func Remove(c *gin.Context) {
	hostID, _ := c.GetQuery("host_id")
	if hostID == "" {
		logger.Log(0, "host id is required")
		utils.ReturnErrorResponse(c,
			errpkg.FormatError(errors.New("host id is required"), errpkg.BadRequest))
		return
	}
	utils.ReturnSuccessResponse(c,
		fmt.Sprintf("unregistered host (%s) successfully", hostID), nil)
}
