package main

import (
	"context"
	"fmt"
	"time"

	"github.com/awebai/aw/chat"
	"github.com/spf13/cobra"
)

var (
	chatWaitSeconds  int
	chatStartConv    bool
	chatCmdGroup     = &cobra.Command{Use: "chat", Short: "Real-time conversations"}
	chatStatusEcho   = func(kind, message string) { fmt.Printf("[%s] %s\n", kind, message) }
	chatLongDeadline = 10 * time.Minute
)

var chatSendAndWaitCmd = &cobra.Command{
	Use:   "send-and-wait <address> <message>",
	Short: "Send a chat message and wait for the reply",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		c, identity, err := identityClient()
		if err != nil {
			return err
		}
		ctx, cancel := context.WithTimeout(context.Background(), chatLongDeadline)
		defer cancel()
		result, err := chat.Send(ctx, c, identity.Address, []string{args[0]}, args[1], chat.SendOptions{
			Wait:              chatWaitSeconds,
			WaitExplicit:      cmd.Flags().Changed("wait"),
			StartConversation: chatStartConv,
		}, chatStatusEcho)
		if err != nil {
			return err
		}
		printChatResult(result)
		return nil
	},
}

var chatSendAndLeaveCmd = &cobra.Command{
	Use:   "send-and-leave <address> <message>",
	Short: "Send a chat message without waiting for a reply",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		c, identity, err := identityClient()
		if err != nil {
			return err
		}
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()
		result, err := chat.Send(ctx, c, identity.Address, []string{args[0]}, args[1], chat.SendOptions{
			Wait:         0,
			WaitExplicit: true,
			Leaving:      true,
		}, chatStatusEcho)
		if err != nil {
			return err
		}
		printChatResult(result)
		return nil
	},
}

var chatPendingCmd = &cobra.Command{
	Use:   "pending",
	Short: "List conversations with unread messages",
	RunE: func(cmd *cobra.Command, args []string) error {
		c, _, err := identityClient()
		if err != nil {
			return err
		}
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		result, err := chat.Pending(ctx, c)
		if err != nil {
			return err
		}
		if len(result.Pending) == 0 {
			fmt.Println("No pending conversations")
			return nil
		}
		for _, p := range result.Pending {
			fmt.Printf("— %s (unread %d, waiting=%v)\n  last from %s: %s\n",
				p.SessionID, p.UnreadCount, p.SenderWaiting, p.LastFrom, p.LastMessage)
		}
		return nil
	},
}

var chatOpenCmd = &cobra.Command{
	Use:   "open <address>",
	Short: "Read unread chat messages from an address",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		c, _, err := identityClient()
		if err != nil {
			return err
		}
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		result, err := chat.Open(ctx, c, args[0])
		if err != nil {
			return err
		}
		for _, m := range result.Messages {
			fmt.Printf("[%s] %s: %s\n", m.Timestamp, chatEventFrom(m), m.Body)
		}
		if len(result.Messages) == 0 {
			fmt.Println("No unread messages.")
		}
		return nil
	},
}

var chatHistoryCmd = &cobra.Command{
	Use:   "history <address>",
	Short: "Show the full conversation with an address",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		c, _, err := identityClient()
		if err != nil {
			return err
		}
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		result, err := chat.History(ctx, c, args[0])
		if err != nil {
			return err
		}
		for _, m := range result.Messages {
			fmt.Printf("[%s] %s: %s\n", m.Timestamp, chatEventFrom(m), m.Body)
		}
		return nil
	},
}

var chatExtendWaitCmd = &cobra.Command{
	Use:   "extend-wait <address> <message>",
	Short: "Ask the waiting party for more time",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		c, _, err := identityClient()
		if err != nil {
			return err
		}
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		result, err := chat.ExtendWait(ctx, c, args[0], args[1])
		if err != nil {
			return err
		}
		fmt.Printf("Wait extended by %d seconds\n", result.ExtendsWaitSeconds)
		return nil
	},
}

func printChatResult(result *chat.SendResult) {
	fmt.Printf("Status: %s (session %s)\n", result.Status, result.SessionID)
	if result.Reply != "" {
		fmt.Printf("Reply: %s\n", result.Reply)
	}
	for _, e := range result.Events {
		if e.Body != "" {
			fmt.Printf("[%s] %s: %s\n", e.Timestamp, chatEventFrom(e), e.Body)
		}
	}
}

func init() {
	chatSendAndWaitCmd.Flags().IntVar(&chatWaitSeconds, "wait", 120, "Seconds to wait for a reply")
	chatSendAndWaitCmd.Flags().BoolVar(&chatStartConv, "start-conversation", false, "First contact: wait up to 5 minutes")
	chatCmdGroup.AddCommand(chatSendAndWaitCmd, chatSendAndLeaveCmd, chatPendingCmd, chatOpenCmd, chatHistoryCmd, chatExtendWaitCmd)
	rootCmd.AddCommand(chatCmdGroup)
}

func chatEventFrom(e chat.Event) string {
	if e.FromAddress != "" {
		return e.FromAddress
	}
	if e.FromAgent != "" {
		return e.FromAgent
	}
	return e.FromDID
}
