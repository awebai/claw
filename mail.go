package main

import (
	"context"
	"fmt"
	"time"

	"github.com/awebai/aw/awid"
	"github.com/spf13/cobra"
)

var (
	mailSendTo           string
	mailSendSubject      string
	mailSendBody         string
	mailSendPriority     string
	mailSendConversation string
	mailInboxShowAll     bool
	mailInboxLimit       int
	mailAckMessageID     string
)

var mailCmd = &cobra.Command{Use: "mail", Short: "Asynchronous durable messages"}

var mailSendCmd = &cobra.Command{
	Use:   "send",
	Short: "Send mail to a federated address (e.g. kate.claweb.ai/buddy)",
	RunE: func(cmd *cobra.Command, args []string) error {
		if mailSendTo == "" && mailSendConversation == "" {
			return fmt.Errorf("--to <address> or --conversation <id> is required")
		}
		if mailSendBody == "" {
			return fmt.Errorf("--body is required")
		}
		c, _, err := identityClient()
		if err != nil {
			return err
		}
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		resp, err := c.SendMessage(ctx, &awid.SendMessageRequest{
			ToAddress:      mailSendTo,
			ConversationID: mailSendConversation,
			Subject:        mailSendSubject,
			Body:           mailSendBody,
			Priority:       awid.MessagePriority(mailSendPriority),
		})
		if err != nil {
			return err
		}
		fmt.Printf("Sent (message_id=%s conversation_id=%s)\n", resp.MessageID, resp.ConversationID)
		return nil
	},
}

var mailInboxCmd = &cobra.Command{
	Use:   "inbox",
	Short: "List inbox messages (unread only by default; reading acknowledges)",
	RunE: func(cmd *cobra.Command, args []string) error {
		c, _, err := identityClient()
		if err != nil {
			return err
		}
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		resp, err := c.Inbox(ctx, awid.InboxParams{
			UnreadOnly: !mailInboxShowAll,
			Limit:      mailInboxLimit,
		})
		if err != nil {
			return err
		}
		if len(resp.Messages) == 0 {
			fmt.Println("No messages.")
			return nil
		}
		for _, m := range resp.Messages {
			from := m.FromAddress
			if from == "" {
				from = m.FromDID
			}
			fmt.Printf("— %s\n  From: %s\n  Subject: %s\n  message_id: %s  conversation_id: %s\n  %s\n",
				m.CreatedAt, from, m.Subject, m.MessageID, m.ConversationID, m.Body)
			if m.ReadAt == nil && m.MessageID != "" {
				_, _ = c.AckMessage(ctx, m.MessageID)
			}
		}
		return nil
	},
}

var mailAckCmd = &cobra.Command{
	Use:   "ack",
	Short: "Mark a message as read",
	RunE: func(cmd *cobra.Command, args []string) error {
		if mailAckMessageID == "" {
			return fmt.Errorf("--message-id is required")
		}
		c, _, err := identityClient()
		if err != nil {
			return err
		}
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		if _, err := c.AckMessage(ctx, mailAckMessageID); err != nil {
			return err
		}
		fmt.Println("Acknowledged.")
		return nil
	},
}

func init() {
	mailSendCmd.Flags().StringVar(&mailSendTo, "to", "", "Recipient address (domain/name)")
	mailSendCmd.Flags().StringVar(&mailSendSubject, "subject", "", "Subject")
	mailSendCmd.Flags().StringVar(&mailSendBody, "body", "", "Message body")
	mailSendCmd.Flags().StringVar(&mailSendPriority, "priority", "normal", "low|normal|high|urgent")
	mailSendCmd.Flags().StringVar(&mailSendConversation, "conversation", "", "Reply within an existing conversation")
	mailInboxCmd.Flags().BoolVar(&mailInboxShowAll, "show-all", false, "Include already-read messages")
	mailInboxCmd.Flags().IntVar(&mailInboxLimit, "limit", 50, "Max messages")
	mailAckCmd.Flags().StringVar(&mailAckMessageID, "message-id", "", "Message to acknowledge")
	mailCmd.AddCommand(mailSendCmd, mailInboxCmd, mailAckCmd)
	rootCmd.AddCommand(mailCmd)
}
