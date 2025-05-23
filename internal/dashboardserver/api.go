package dashboardserver

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"path"
	"time"

	"github.com/gin-contrib/static"
	"github.com/gin-gonic/gin"
	"github.com/spf13/viper"
	"github.com/turbot/pipe-fittings/v2/constants"
	"github.com/turbot/pipe-fittings/v2/error_helpers"
	"github.com/turbot/pipe-fittings/v2/filepaths"
	"gopkg.in/olahol/melody.v1"
)

func startAPIAsync(ctx context.Context, webSocket *melody.Melody) chan struct{} {
	doneChan := make(chan struct{})

	go func() {
		gin.SetMode(gin.ReleaseMode)
		router := gin.New()
		// only add the Recovery middleware
		router.Use(gin.Recovery())

		assetsDirectory := filepaths.EnsureDashboardAssetsDir()

		router.Use(static.Serve("/", static.LocalFile(assetsDirectory, true)))

		router.GET("/ws", func(c *gin.Context) {
			webSocket.HandleRequest(c.Writer, c.Request) //nolint:errcheck // TODO: fix this
		})

		router.NoRoute(func(c *gin.Context) {
			// https://stackoverflow.com/questions/49547/how-do-we-control-web-page-caching-across-all-browsers
			c.Header("Cache-Control", "no-cache, no-store, must-revalidate") // HTTP 1.1.
			c.Header("Pragma", "no-cache")                                   // HTTP 1.0.
			c.Header("Expires", "0")                                         // Proxies.
			c.File(path.Join(assetsDirectory, "index.html"))
		})

		dashboardServerPort := viper.GetInt(constants.ArgPort)

		dashboardServerListen := "localhost"
		if viper.GetString(constants.ArgListen) == string(ListenTypeNetwork) {
			dashboardServerListen = ""
		}

		//nolint: gosec // TODO FIX ME
		srv := &http.Server{
			Addr:    fmt.Sprintf("%s:%d", dashboardServerListen, dashboardServerPort),
			Handler: router,
		}

		go func() {
			// service connections
			if err := srv.ListenAndServe(); err != nil {
				slog.Warn("listen error", "error", err)
			}
		}()

		OutputReady(ctx, fmt.Sprintf("Dashboard server started on %d and listening on %s", dashboardServerPort, viper.GetString(constants.ArgListen)))
		OutputMessage(ctx, fmt.Sprintf("Visit http://localhost:%d", dashboardServerPort))
		OutputMessage(ctx, "Press Ctrl+C to exit")
		<-ctx.Done()
		slog.Debug("Shutdown Server…")

		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if err := srv.Shutdown(shutdownCtx); err != nil {
			error_helpers.ShowErrorWithMessage(ctx, err, "Server shutdown failed")
		}
		slog.Debug("Server exiting")

		// indicate the API server is done
		doneChan <- struct{}{}
	}()

	return doneChan
}
