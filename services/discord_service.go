package services

import (
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
	"xand/models"
)

type DiscordBotService struct {
	session   *discordgo.Session
	channelID string
	botID     string
	enabled   bool
}

func NewDiscordBotService(token string, channelID string) (*DiscordBotService, error) {
	if token == "" {
		log.Println("Discord bot token not provided, Discord notifications disabled")
		return &DiscordBotService{enabled: false}, nil
	}

	if channelID == "" {
		log.Println("Discord channel ID not provided, Discord notifications disabled")
		return &DiscordBotService{enabled: false}, nil
	}

	// Create Discord session with Bot prefix
	session, err := discordgo.New("Bot " + token)
	if err != nil {
		return nil, fmt.Errorf("failed to create Discord session: %w", err)
	}

	// Get bot user information
	user, err := session.User("@me")
	if err != nil {
		return nil, fmt.Errorf("failed to get bot user: %w", err)
	}

	botService := &DiscordBotService{
		session:   session,
		channelID: channelID,
		botID:     user.ID,
		enabled:   true,
	}

	// Add message handler for potential interactive features
	session.AddHandler(botService.messageHandler)

	// Open websocket connection to Discord
	err = session.Open()
	if err != nil {
		return nil, fmt.Errorf("failed to open Discord connection: %w", err)
	}

	log.Printf("Discord bot connected successfully! Bot ID: %s, Channel: %s", user.ID, channelID)

	return botService, nil
}

func (d *DiscordBotService) Close() {
	if d.enabled && d.session != nil {
		log.Println("Closing Discord bot connection...")
		d.session.Close()
	}
}

// messageHandler handles incoming Discord messages (for potential interactive features)
func (d *DiscordBotService) messageHandler(s *discordgo.Session, m *discordgo.MessageCreate) {
	// Ignore messages from the bot itself
	if m.Author.ID == d.botID {
		return
	}

	// Only respond in the configured channel
	if m.ChannelID != d.channelID {
		return
	}

	// Simple command handling for future expansion
	if strings.HasPrefix(m.Content, "!xand") {
		args := strings.Fields(m.Content)
		if len(args) < 2 {
			return
		}

		cmd := args[1]
		switch cmd {
		case "ping":
			s.ChannelMessageSend(m.ChannelID, "🏓 Pong! Xandeum Analytics bot is online!")
		case "help":
			helpMsg := "**Xandeum Analytics Bot Commands:**\n" +
				"`!xand ping` - Check if bot is online\n" +
				"`!xand help` - Show this help message\n" +
				"`!xand status` - Get current network status"
			s.ChannelMessageSend(m.ChannelID, helpMsg)
		case "status":
			s.ChannelMessageSend(m.ChannelID, "Use the API endpoints to get detailed network status!")
		default:
			s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("Unknown command: `%s`. Try `!xand help`", cmd))
		}
	}
}

// SendAlert sends an alert to Discord
func (d *DiscordBotService) SendAlert(alert *models.Alert, context map[string]interface{}) error {
	if !d.enabled {
		return fmt.Errorf("Discord bot not enabled")
	}

	embed := d.createAlertEmbed(alert, context)
	
	_, err := d.session.ChannelMessageSendEmbed(d.channelID, embed)
	if err != nil {
		return fmt.Errorf("failed to send Discord message: %w", err)
	}

	log.Printf("Alert sent to Discord: %s", alert.Name)
	return nil
}

// SendTestAlert sends a test alert
func (d *DiscordBotService) SendTestAlert(alert *models.Alert, context map[string]interface{}) error {
	if !d.enabled {
		return fmt.Errorf("Discord bot not enabled")
	}

	embed := d.createTestAlertEmbed(alert, context)
	
	_, err := d.session.ChannelMessageSendEmbed(d.channelID, embed)
	if err != nil {
		return fmt.Errorf("failed to send Discord test message: %w", err)
	}

	log.Printf("Test alert sent to Discord: %s", alert.Name)
	return nil
}

func (d *DiscordBotService) createAlertEmbed(alert *models.Alert, context map[string]interface{}) *discordgo.MessageEmbed {
	// Determine color based on rule type
	color := d.getColorForRuleType(alert.RuleType)

	// Build fields from context
	fields := d.buildFields(alert.RuleType, context)

	embed := &discordgo.MessageEmbed{
		Title:       "🚨 " + alert.Name,
		Description: alert.Description,
		Color:       color,
		Fields:      fields,
		Footer: &discordgo.MessageEmbedFooter{
			Text: fmt.Sprintf("Alert ID: %s", alert.ID),
		},
		Timestamp: time.Now().Format(time.RFC3339),
	}

	return embed
}

func (d *DiscordBotService) createTestAlertEmbed(alert *models.Alert, context map[string]interface{}) *discordgo.MessageEmbed {
	fields := d.buildFields(alert.RuleType, context)

	embed := &discordgo.MessageEmbed{
		Title:       "🧪 Test Alert: " + alert.Name,
		Description: alert.Description + "\n\n*This is a test alert. No actual alert condition was triggered.*",
		Color:       3447003, // Blue for test
		Fields:      fields,
		Footer: &discordgo.MessageEmbedFooter{
			Text: "Test Alert",
		},
		Timestamp: time.Now().Format(time.RFC3339),
	}

	return embed
}

func (d *DiscordBotService) getColorForRuleType(ruleType string) int {
	switch ruleType {
	case "node_status":
		return 15158332 // Red
	case "network_health":
		return 15844367 // Gold/Yellow
	case "storage_threshold":
		return 10181046 // Purple
	case "latency_spike":
		return 15105570 // Orange
	default:
		return 3447003 // Blue
	}
}

func (d *DiscordBotService) buildFields(ruleType string, context map[string]interface{}) []*discordgo.MessageEmbedField {
	fields := []*discordgo.MessageEmbedField{
		{
			Name:   "Rule Type",
			Value:  d.formatRuleType(ruleType),
			Inline: true,
		},
		{
			Name:   "Triggered At",
			Value:  time.Now().Format("2006-01-02 15:04:05 MST"),
			Inline: true,
		},
	}

	// Add context-specific fields
	switch ruleType {
	case "node_status":
		if nodeID, ok := context["node_id"].(string); ok {
			fields = append(fields, &discordgo.MessageEmbedField{
				Name:   "Node ID",
				Value:  nodeID,
				Inline: false,
			})
		}
		if nodeAddr, ok := context["node_address"].(string); ok {
			fields = append(fields, &discordgo.MessageEmbedField{
				Name:   "Node Address",
				Value:  nodeAddr,
				Inline: true,
			})
		}
		if status, ok := context["status"].(string); ok {
			fields = append(fields, &discordgo.MessageEmbedField{
				Name:   "Status",
				Value:  strings.ToUpper(status),
				Inline: true,
			})
		}

	case "network_health":
		if health, ok := context["network_health"].(float64); ok {
			fields = append(fields, &discordgo.MessageEmbedField{
				Name:   "Network Health",
				Value:  fmt.Sprintf("%.2f%%", health),
				Inline: true,
			})
		}
		if threshold, ok := context["threshold"].(float64); ok {
			fields = append(fields, &discordgo.MessageEmbedField{
				Name:   "Threshold",
				Value:  fmt.Sprintf("%.2f%%", threshold),
				Inline: true,
			})
		}

	case "storage_threshold":
		if storageType, ok := context["storage_type"].(string); ok {
			fields = append(fields, &discordgo.MessageEmbedField{
				Name:   "Storage Type",
				Value:  strings.Title(storageType),
				Inline: true,
			})
		}
		if value, ok := context["value"].(float64); ok {
			var valueStr string
			if context["storage_type"] == "percent" {
				valueStr = fmt.Sprintf("%.2f%%", value)
			} else {
				valueStr = fmt.Sprintf("%.2f PB", value)
			}
			fields = append(fields, &discordgo.MessageEmbedField{
				Name:   "Current Value",
				Value:  valueStr,
				Inline: true,
			})
		}
		if threshold, ok := context["threshold"].(float64); ok {
			var thresholdStr string
			if context["storage_type"] == "percent" {
				thresholdStr = fmt.Sprintf("%.2f%%", threshold)
			} else {
				thresholdStr = fmt.Sprintf("%.2f PB", threshold)
			}
			fields = append(fields, &discordgo.MessageEmbedField{
				Name:   "Threshold",
				Value:  thresholdStr,
				Inline: true,
			})
		}

	case "latency_spike":
		if nodeID, ok := context["node_id"].(string); ok {
			fields = append(fields, &discordgo.MessageEmbedField{
				Name:   "Node ID",
				Value:  nodeID,
				Inline: false,
			})
		}
		if latency, ok := context["latency"].(int64); ok {
			fields = append(fields, &discordgo.MessageEmbedField{
				Name:   "Current Latency",
				Value:  fmt.Sprintf("%d ms", latency),
				Inline: true,
			})
		}
		if threshold, ok := context["threshold"].(float64); ok {
			fields = append(fields, &discordgo.MessageEmbedField{
				Name:   "Threshold",
				Value:  fmt.Sprintf("%.0f ms", threshold),
				Inline: true,
			})
		}
	}

	return fields
}

func (d *DiscordBotService) formatRuleType(ruleType string) string {
	switch ruleType {
	case "node_status":
		return "Node Status"
	case "network_health":
		return "Network Health"
	case "storage_threshold":
		return "Storage Threshold"
	case "latency_spike":
		return "Latency Spike"
	default:
		return ruleType
	}
}

// SendHealthReport sends a periodic health report (optional feature)
func (d *DiscordBotService) SendHealthReport(stats map[string]interface{}) error {
	if !d.enabled {
		return fmt.Errorf("Discord bot not enabled")
	}

	embed := &discordgo.MessageEmbed{
		Title:       "📊 Network Health Report",
		Description: "Daily network statistics summary",
		Color:       3066993, // Green
		Fields: []*discordgo.MessageEmbedField{
			{
				Name:   "Online Nodes",
				Value:  fmt.Sprintf("%v", stats["online_nodes"]),
				Inline: true,
			},
			{
				Name:   "Total Nodes",
				Value:  fmt.Sprintf("%v", stats["total_nodes"]),
				Inline: true,
			},
			{
				Name:   "Network Health",
				Value:  fmt.Sprintf("%.2f%%", stats["network_health"]),
				Inline: true,
			},
			{
				Name:   "Storage Used",
				Value:  fmt.Sprintf("%.2f PB", stats["used_storage"]),
				Inline: true,
			},
			{
				Name:   "Total Storage",
				Value:  fmt.Sprintf("%.2f PB", stats["total_storage"]),
				Inline: true,
			},
		},
		Timestamp: time.Now().Format(time.RFC3339),
	}

	_, err := d.session.ChannelMessageSendEmbed(d.channelID, embed)
	if err != nil {
		return fmt.Errorf("failed to send health report: %w", err)
	}

	log.Println("Health report sent to Discord")
	return nil
}

// SendMessage sends a simple text message to the channel
func (d *DiscordBotService) SendMessage(message string) error {
	if !d.enabled {
		return fmt.Errorf("Discord bot not enabled")
	}

	_, err := d.session.ChannelMessageSend(d.channelID, message)
	if err != nil {
		return fmt.Errorf("failed to send message: %w", err)
	}

	return nil
}