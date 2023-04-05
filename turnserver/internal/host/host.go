package host

import (
	"errors"
	"fmt"

	"github.com/gin-gonic/gin"

	"github.com/gravitl/netmaker/logger"
	"github.com/gravitl/netmaker/turnserver/internal/auth"
	errpkg "github.com/gravitl/netmaker/turnserver/internal/errors"
	"github.com/gravitl/netmaker/turnserver/internal/models"
	"github.com/gravitl/netmaker/turnserver/internal/utils"
)

func Register(c *gin.Context) {
	req := models.HostRegister{}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.ReturnErrorResponse(c, errpkg.FormatError(err, errpkg.Internal))
		return
	}
	auth.RegisterNewHostWithTurn(req.HostID, req.HostPassHash)
	utils.ReturnSuccessResponse(c,
		fmt.Sprintf("registred host (%s) successfully", req.HostID), nil)
}

func Remove(c *gin.Context) {
	hostID, _ := c.GetQuery("host_id")
	if hostID == "" {
		logger.Log(0, "host id is required")
		utils.ReturnErrorResponse(c,
			errpkg.FormatError(errors.New("host id is required"), errpkg.BadRequest))
		return
	}
	utils.ReturnSuccessResponse(c,
		fmt.Sprintf("unregistred host (%s) successfully", hostID), nil)
}
