package proxy

import (
	"github.com/gin-contrib/cors"
	"github.com/gin-contrib/gzip"
	"github.com/gin-gonic/gin"
	"github.com/splitio/split-synchronizer/splitio/web/admin"
	"github.com/splitio/split-synchronizer/splitio/web/middleware"
)

// ProxyOptions struct to set options for Proxy mode.
type ProxyOptions struct {
	Port                      string
	AdminPort                 int
	AdminUsername             string
	AdminPassword             string
	APIKeys                   []string
	ImpressionListenerEnabled bool
	DebugOn                   bool
}

// Run runs the proxy server
func Run(options *ProxyOptions) {
	if !options.DebugOn {
		gin.SetMode(gin.ReleaseMode)
	}

	router := gin.New()
	router.Use(gin.Recovery())

	//CORS - Allows all origins
	corsConfig := cors.DefaultConfig()
	corsConfig.AllowAllOrigins = true
	corsConfig.AllowHeaders = []string{
		"Origin",
		"Content-Length",
		"Content-Type",
		"SplitSDKMachineName",
		"SplitSDKMachineIP",
		"SplitSDKVersion",
		"Authorization"}
	router.Use(cors.New(corsConfig))

	router.Use(gzip.Gzip(gzip.DefaultCompression))
	router.Use(middleware.Logger())
	router.Use(middleware.ValidateAPIKeys(options.APIKeys))

	// running admin endpoints
	go func() {
		// WebAdmin configuration
		waOptions := &admin.WebAdminOptions{
			Port:          options.AdminPort,
			AdminUsername: options.AdminUsername,
			AdminPassword: options.AdminPassword,
			DebugOn:       options.DebugOn,
		}

		waServer := admin.NewWebAdminServer(waOptions)

		waServer.Router().GET("/admin/healthcheck", healthCheck)
		waServer.Router().GET("/admin/stats", showStats)
		waServer.Router().GET("/admin/dashboard", showDashboard)
		waServer.Router().GET("/admin/dashboard/segmentKeys/:segment", showDashboardSegmentKeys)

		waServer.Run()
	}()

	// API routes
	api := router.Group("/api")
	{
		api.GET("/splitChanges", splitChanges)
		api.GET("/segmentChanges/:name", segmentChanges)
		api.GET("/mySegments/:key", mySegments)
		api.POST("/testImpressions/bulk", postImpressionBulk(options.ImpressionListenerEnabled))
		api.POST("/metrics/times", postMetricsTimes)
		api.POST("/metrics/counters", postMetricsCounters)
		api.POST("/metrics/gauge", postMetricsGauge)
		api.POST("/metrics/time", postMetricsTime)
		api.POST("/metrics/counter", postMetricsCounter)
	}
	router.Run(options.Port)
}
