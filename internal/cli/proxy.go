package cli

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/mattwalters/bubble/internal/proxy"
	"github.com/mattwalters/bubble/internal/ui"
	"github.com/spf13/cobra"
)

var (
	proxyRunPort int
)

var proxyCmd = &cobra.Command{
	Use:   "proxy",
	Short: "Manage the built-in HTTP and WebSocket reverse proxy",
}

var proxyStartCmd = &cobra.Command{
	Use:   "start",
	Short: "Start the proxy background daemon",
	RunE: func(cmd *cobra.Command, args []string) error {
		port := proxy.ConfiguredPort()
		if err := proxy.StartDaemon(port); err != nil {
			return err
		}
		ui.PrintSuccess(fmt.Sprintf("Bubble proxy started on port %d", port))
		return nil
	},
}

var proxyStopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stop the running proxy daemon",
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := proxy.StopDaemon(); err != nil {
			return err
		}
		ui.PrintSuccess("Bubble proxy stopped")
		return nil
	},
}

var proxyStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Check the status of the proxy daemon",
	RunE: func(cmd *cobra.Command, args []string) error {
		port := proxy.ConfiguredPort()
		healthy, pid := proxy.DaemonStatus(port)
		if healthy {
			ui.PrintSuccess(fmt.Sprintf("Bubble proxy is running (PID %d) on http://127.0.0.1:%d", pid, port))
		} else {
			ui.PrintWarning(fmt.Sprintf("Bubble proxy is NOT running on port %d", port))
		}
		return nil
	},
}

var proxyRunCmd = &cobra.Command{
	Use:    "run",
	Short:  "Run the proxy server in the foreground",
	Hidden: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		if proxyRunPort <= 0 {
			proxyRunPort = proxy.ConfiguredPort()
		}

		srv := proxy.NewServer(proxyRunPort)
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
		go func() {
			<-sigChan
			cancel()
		}()

		fmt.Printf("Starting Bubble proxy on 127.0.0.1:%d\n", proxyRunPort)
		return srv.Start(ctx)
	},
}

func init() {
	proxyRunCmd.Flags().IntVar(&proxyRunPort, "port", 0, "Port to listen on")

	proxyCmd.AddCommand(proxyStartCmd)
	proxyCmd.AddCommand(proxyStopCmd)
	proxyCmd.AddCommand(proxyStatusCmd)
	proxyCmd.AddCommand(proxyRunCmd)
}
