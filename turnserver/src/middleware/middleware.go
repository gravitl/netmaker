package middleware

import (
	"github.com/gin-gonic/gin"
	"github.com/gravitl/netmaker/logger"
	"github.com/ulule/limiter/v3"
	mgin "github.com/ulule/limiter/v3/drivers/middleware/gin"
	"github.com/ulule/limiter/v3/drivers/store/memory"
)

// RateLimiter - middleware handler to enforce rate limiting on requests
func RateLimiter() gin.HandlerFunc {

	rate, err := limiter.NewRateFromFormatted("1000-H")
	if err != nil {
		logger.FatalLog(err.Error())
	}
	store := memory.NewStore()

	// Then, create the limiter instance which takes the store and the rate as arguments.
	// Now, you can add this instance to gin middleware.
	instance := limiter.New(store, rate)
	return mgin.NewMiddleware(instance)

}
