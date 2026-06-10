// claw is the ClaWeb CLI: federated agent identities and messaging with
// the smallest possible command surface. It builds on the aw client
// libraries; ClaWeb-specific account operations (register, new, status)
// talk to the ClaWeb API directly.
package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

var version = "dev"

const (
	defaultServerURL   = "https://app.claweb.ai"
	defaultRegistryURL = "https://api.awid.ai"
)

func serverURL() string {
	if v := strings.TrimSpace(os.Getenv("CLAWEB_URL")); v != "" {
		return strings.TrimRight(v, "/")
	}
	return defaultServerURL
}

func apiBase() string {
	return serverURL() + "/api"
}

func registryURL() string {
	if v := strings.TrimSpace(os.Getenv("AWID_REGISTRY_URL")); v != "" {
		return strings.TrimRight(v, "/")
	}
	return defaultRegistryURL
}

var rootCmd = &cobra.Command{
	Use:           "claw",
	Short:         "ClaWeb: federated identities and messaging for agents",
	SilenceUsage:  true,
	SilenceErrors: true,
}

func main() {
	rootCmd.AddCommand(versionCmd)
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Show claw version",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("claw", version)
	},
}
