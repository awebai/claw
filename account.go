package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/awebai/aw/awconfig"
	"github.com/awebai/aw/awid"
	"github.com/spf13/cobra"
)

var registerCmd = &cobra.Command{
	Use:   "register <slug>",
	Short: "Register a ClaWeb account (anonymous; your slug becomes <slug>.claweb.ai)",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		status, out, err := clawebRequest("POST", "/v1/register",
			map[string]string{"slug": args[0]}, "")
		if err != nil {
			return err
		}
		if status != 201 {
			return apiError(status, out)
		}
		slug, _ := out["slug"].(string)
		namespace, _ := out["namespace"].(string)
		secret, _ := out["account_secret"].(string)
		if err := saveAccount(slug, namespace, secret); err != nil {
			return fmt.Errorf("account created but saving the secret failed: %w; secret: %s", err, secret)
		}
		sp, _ := secretPath()
		fmt.Printf("Registered %s\n", namespace)
		fmt.Printf("Account secret stored at %s\n", sp)
		fmt.Println("The secret cannot be recovered. Keep that file safe.")
		fmt.Println("Next: claw new <name>   (creates an identity like " + namespace + "/<name>)")
		return nil
	},
}

var newCmd = &cobra.Command{
	Use:   "new <name>",
	Short: "Create an identity in this directory: keys stay local, ClaWeb assigns <slug>.claweb.ai/<name>",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]
		account, err := loadAccount()
		if err != nil {
			return err
		}
		secret, err := loadSecret()
		if err != nil {
			return err
		}
		wd, err := os.Getwd()
		if err != nil {
			return err
		}
		identityPath := filepath.Join(wd, awconfig.DefaultWorktreeIdentityRelativePath())
		if _, err := os.Stat(identityPath); err == nil {
			return errors.New("this directory already has an identity (.aw/identity.yaml)")
		}

		pub, key, err := awid.GenerateKeypair()
		if err != nil {
			return err
		}
		didKey := awid.ComputeDIDKey(pub)
		didAW := awid.ComputeStableID(pub)

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		registry := awid.NewAWIDRegistryClient(nil, nil)
		if err := registry.SetFallbackRegistryURL(registryURL()); err != nil {
			return err
		}
		if _, err := registry.RegisterIdentity(ctx, registryURL(), didKey, didAW, key); err != nil {
			var already *awid.AlreadyRegisteredError
			if !errors.As(err, &already) {
				return fmt.Errorf("register DID at the registry: %w", err)
			}
		}

		status, out, err := clawebRequest("POST", "/v1/identities", map[string]string{
			"name": name, "did_aw": didAW, "did_key": didKey,
		}, secret)
		if err != nil {
			return err
		}
		if status != 201 {
			return apiError(status, out)
		}
		address, _ := out["address"].(string)

		if err := awid.SaveSigningKey(awconfig.WorktreeSigningKeyPath(wd), key); err != nil {
			return err
		}
		if err := awconfig.SaveWorktreeIdentityTo(identityPath, &awconfig.WorktreeIdentity{
			DID:            didKey,
			StableID:       didAW,
			Address:        address,
			Custody:        awid.CustodySelf,
			Lifetime:       awid.LifetimePersistent,
			RegistryURL:    registryURL(),
			RegistryStatus: "registered",
			CreatedAt:      time.Now().UTC().Format(time.RFC3339),
		}); err != nil {
			return err
		}
		fmt.Printf("Created %s\n", address)
		fmt.Printf("Keys in %s — they never leave this machine.\n", filepath.Join(wd, ".aw"))
		_ = account
		return nil
	},
}

var whoamiCmd = &cobra.Command{
	Use:   "whoami",
	Short: "Show the identity in this directory and your account",
	RunE: func(cmd *cobra.Command, args []string) error {
		wd, _ := os.Getwd()
		identity, err := awconfig.ResolveIdentity(wd)
		if err == nil {
			fmt.Printf("Identity: %s\n", identity.Address)
			fmt.Printf("did:aw:   %s\n", identity.StableID)
			fmt.Printf("did:key:  %s\n", identity.DID)
		} else {
			fmt.Println("Identity: none in this directory (run `claw new <name>`)")
		}
		if account, err := loadAccount(); err == nil {
			fmt.Printf("Account:  %s (%s)\n", account.Slug, account.Namespace)
		} else {
			fmt.Println("Account:  none (run `claw register <slug>`)")
		}
		return nil
	},
}

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show tier, identity count, and today's message usage",
	RunE: func(cmd *cobra.Command, args []string) error {
		secret, err := loadSecret()
		if err != nil {
			return err
		}
		status, out, err := clawebRequest("GET", "/v1/status", nil, secret)
		if err != nil {
			return err
		}
		if status != 200 {
			return apiError(status, out)
		}
		fmt.Printf("Account:    %s (%s)\n", out["slug"], out["namespace"])
		fmt.Printf("Tier:       %s\n", out["tier"])
		if ids, ok := out["identities"].(map[string]any); ok {
			fmt.Printf("Identities: %v / %v\n", ids["used"], ids["limit"])
		}
		if msgs, ok := out["messages_today"].(map[string]any); ok {
			fmt.Printf("Sent today: %v / %v (resets %v)\n", msgs["used"], msgs["limit"], msgs["resets_at"])
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(registerCmd, newCmd, whoamiCmd, statusCmd)
}
