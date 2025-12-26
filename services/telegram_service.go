package services

// import (
// 	"fmt"
// 	"log"
// 	"strings"
// 	"time"
//
// 	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
// 	"xand/models"
// )

// type TelegramBotService struct {
// 	bot     *tgbotapi.BotAPI
// 	chatID  int64
// 	enabled bool
// }

// func NewTelegramBotService(token string, chatID int64) (*TelegramBotService, error) {
// 	if token == "" {
// 		log.Println("Telegram bot token not provided, Telegram notifications disabled")
// 		return &TelegramBotService{enabled: false}, nil
// 	}

// 	if chatID == 0 {
// 		log.Println("Telegram chat ID not provided, Telegram notifications disabled")
// 		return &TelegramBotService{enabled: false}, nil
// 	}

// 	// Create Telegram bot
// 	bot, err := tgbotapi.NewBotAPI(token)
// 	if err != nil {
// 		return nil, fmt.Errorf("failed to create Telegram bot: %w", err)
// 	}

// 	bot.Debug = false
// 	log.Printf("Telegram bot authorized: %s", bot.Self.UserName)

// 	return &TelegramBotService{
// 		bot:     bot,
// 		chatID:  chatID,
// 		enabled: true,
// 	}, nil
// }

// // SendAlert sends an alert to Telegram
// func (t *TelegramBotService) SendAlert(alert *models.Alert, context map[string]interface{}) error {
// 	if !t.enabled {
// 		return fmt.Errorf("Telegram bot not enabled")
// 	}

// 	message := t.formatAlertMessage(alert, context)

// 	msg := tgbotapi.NewMessage(t.chatID, message)
// 	msg.ParseMode = "MarkdownV2"

// 	_, err := t.bot.Send(msg)
// 	if err != nil {
// 		return fmt.Errorf("failed to send Telegram message: %w", err)
// 	}

// 	return nil
// }

// // SendTestAlert sends a test alert
// func (t *TelegramBotService) SendTestAlert(alert *models.Alert, context map[string]interface{}) error {
// 	if !t.enabled {
// 		return fmt.Errorf("Telegram bot not enabled")
// 	}

// 	message := t.formatTestAlertMessage(alert, context)

// 	msg := tgbotapi.NewMessage(t.chatID, message)
// 	msg.ParseMode = "MarkdownV2"

// 	_, err := t.bot.Send(msg)
// 	if err != nil {
// 		return fmt.Errorf("failed to send Telegram test message: %w", err)
// 	}

// 	return nil
// }

// func (t *TelegramBotService) formatAlertMessage(alert *models.Alert, context map[string]interface{}) string {
// 	var sb strings.Builder

// 	// Header with emoji based on rule type
// 	emoji := t.getEmojiForRuleType(alert.RuleType)
// 	sb.WriteString(fmt.Sprintf("*%s %s*\n\n", emoji, t.escapeMarkdown(alert.Name)))

// 	// Description
// 	sb.WriteString(fmt.Sprintf("%s\n\n", t.escapeMarkdown(alert.Description)))

// 	// Rule Type
// 	sb.WriteString(fmt.Sprintf("*Rule Type:* %s\n", t.escapeMarkdown(t.formatRuleType(alert.RuleType))))
// 	sb.WriteString(fmt.Sprintf("*Triggered At:* %s\n\n", t.escapeMarkdown(time.Now().Format("2006-01-02 15:04:05 MST"))))

// 	// Context details
// 	sb.WriteString(t.formatContextDetails(alert.RuleType, context))

// 	// Footer
// 	sb.WriteString(fmt.Sprintf("\n_Alert ID: %s_", t.escapeMarkdown(alert.ID)))

// 	return sb.String()
// }

// func (t *TelegramBotService) formatTestAlertMessage(alert *models.Alert, context map[string]interface{}) string {
// 	var sb strings.Builder

// 	// Header
// 	sb.WriteString(fmt.Sprintf("*🧪 Test Alert: %s*\n\n", t.escapeMarkdown(alert.Name)))

// 	// Description
// 	sb.WriteString(fmt.Sprintf("%s\n\n", t.escapeMarkdown(alert.Description)))
// 	sb.WriteString("_This is a test alert\\. No actual alert condition was triggered\\._\n\n")

// 	// Rule Type
// 	sb.WriteString(fmt.Sprintf("*Rule Type:* %s\n\n", t.escapeMarkdown(t.formatRuleType(alert.RuleType))))

// 	// Context details
// 	sb.WriteString(t.formatContextDetails(alert.RuleType, context))

// 	return sb.String()
// }

// func (t *TelegramBotService) formatContextDetails(ruleType string, context map[string]interface{}) string {
// 	var sb strings.Builder

// 	switch ruleType {
// 	case "node_status":
// 		if nodeID, ok := context["node_id"].(string); ok {
// 			sb.WriteString(fmt.Sprintf("*Node ID:* `%s`\n", t.escapeMarkdown(nodeID)))
// 		}
// 		if nodeAddr, ok := context["node_address"].(string); ok {
// 			sb.WriteString(fmt.Sprintf("*Address:* `%s`\n", t.escapeMarkdown(nodeAddr)))
// 		}
// 		if status, ok := context["status"].(string); ok {
// 			sb.WriteString(fmt.Sprintf("*Status:* %s\n", t.escapeMarkdown(strings.ToUpper(status))))
// 		}

// 	case "network_health":
// 		if health, ok := context["network_health"].(float64); ok {
// 			sb.WriteString(fmt.Sprintf("*Network Health:* %.2f%%\n", health))
// 		}
// 		if threshold, ok := context["threshold"].(float64); ok {
// 			sb.WriteString(fmt.Sprintf("*Threshold:* %.2f%%\n", threshold))
// 		}

// 	case "storage_threshold":
// 		if storageType, ok := context["storage_type"].(string); ok {
// 			sb.WriteString(fmt.Sprintf("*Storage Type:* %s\n", t.escapeMarkdown(strings.Title(storageType))))
// 		}
// 		if value, ok := context["value"].(float64); ok {
// 			if context["storage_type"] == "percent" {
// 				sb.WriteString(fmt.Sprintf("*Current Value:* %.2f%%\n", value))
// 			} else {
// 				sb.WriteString(fmt.Sprintf("*Current Value:* %.2f PB\n", value))
// 			}
// 		}
// 		if threshold, ok := context["threshold"].(float64); ok {
// 			if context["storage_type"] == "percent" {
// 				sb.WriteString(fmt.Sprintf("*Threshold:* %.2f%%\n", threshold))
// 			} else {
// 				sb.WriteString(fmt.Sprintf("*Threshold:* %.2f PB\n", threshold))
// 			}
// 		}

// 	case "latency_spike":
// 		if nodeID, ok := context["node_id"].(string); ok {
// 			sb.WriteString(fmt.Sprintf("*Node ID:* `%s`\n", t.escapeMarkdown(nodeID)))
// 		}
// 		if latency, ok := context["latency"].(int64); ok {
// 			sb.WriteString(fmt.Sprintf("*Current Latency:* %d ms\n", latency))
// 		}
// 		if threshold, ok := context["threshold"].(float64); ok {
// 			sb.WriteString(fmt.Sprintf("*Threshold:* %.0f ms\n", threshold))
// 		}
// 	}

// 	return sb.String()
// }

// func (t *TelegramBotService) getEmojiForRuleType(ruleType string) string {
// 	switch ruleType {
// 	case "node_status":
// 		return "🚨"
// 	case "network_health":
// 		return "⚠️"
// 	case "storage_threshold":
// 		return "💾"
// 	case "latency_spike":
// 		return "⚡"
// 	default:
// 		return "📢"
// 	}
// }

// func (t *TelegramBotService) formatRuleType(ruleType string) string {
// 	switch ruleType {
// 	case "node_status":
// 		return "Node Status"
// 	case "network_health":
// 		return "Network Health"
// 	case "storage_threshold":
// 		return "Storage Threshold"
// 	case "latency_spike":
// 		return "Latency Spike"
// 	default:
// 		return ruleType
// 	}
// }

// // escapeMarkdown escapes special characters for Telegram MarkdownV2
// func (t *TelegramBotService) escapeMarkdown(text string) string {
// 	replacer := strings.NewReplacer(
// 		"_", "\\_",
// 		"*", "\\*",
// 		"[", "\\[",
// 		"]", "\\]",
// 		"(", "\\(",
// 		")", "\\)",
// 		"~", "\\~",
// 		"`", "\\`",
// 		">", "\\>",
// 		"#", "\\#",
// 		"+", "\\+",
// 		"-", "\\-",
// 		"=", "\\=",
// 		"|", "\\|",
// 		"{", "\\{",
// 		"}", "\\}",
// 		".", "\\.",
// 		"!", "\\!",
// 	)
// 	return replacer.Replace(text)
// }

// // SendHealthReport sends a periodic health report
// func (t *TelegramBotService) SendHealthReport(stats map[string]interface{}) error {
// 	if !t.enabled {
// 		return fmt.Errorf("Telegram bot not enabled")
// 	}

// 	var sb strings.Builder
// 	sb.WriteString("*📊 Network Health Report*\n\n")
// 	sb.WriteString("_Daily network statistics summary_\n\n")

// 	sb.WriteString(fmt.Sprintf("*Online Nodes:* %v\n", stats["online_nodes"]))
// 	sb.WriteString(fmt.Sprintf("*Total Nodes:* %v\n", stats["total_nodes"]))
// 	sb.WriteString(fmt.Sprintf("*Network Health:* %.2f%%\n", stats["network_health"]))
// 	sb.WriteString(fmt.Sprintf("*Storage Used:* %.2f PB\n", stats["used_storage"]))
// 	sb.WriteString(fmt.Sprintf("*Total Storage:* %.2f PB\n", stats["total_storage"]))

// 	msg := tgbotapi.NewMessage(t.chatID, sb.String())
// 	msg.ParseMode = "MarkdownV2"

// 	_, err := t.bot.Send(msg)
// 	return err
// }