// package handlers

// import (
// 	"net/http"
// 	"strconv"

// 	"github.com/labstack/echo/v4"

// 	"xand/models"
// 	"xand/services"
// )

// // AlertHandlers manages alert-related endpoints
// type AlertHandlers struct {
// 	alertService *services.AlertService
// }

// func NewAlertHandlers(alertService *services.AlertService) *AlertHandlers {
// 	return &AlertHandlers{
// 		alertService: alertService,
// 	}
// }

// // CreateAlert godoc
// func (ah *AlertHandlers) CreateAlert(c echo.Context) error {
// 	var alert models.Alert
// 	if err := c.Bind(&alert); err != nil {
// 		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request body"})
// 	}

// 	if err := ah.alertService.CreateAlert(&alert); err != nil {
// 		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
// 	}

// 	return c.JSON(http.StatusCreated, alert)
// }

// // ListAlerts godoc
// func (ah *AlertHandlers) ListAlerts(c echo.Context) error {
// 	alerts := ah.alertService.ListAlerts()
// 	return c.JSON(http.StatusOK, alerts)
// }

// // GetAlert godoc
// func (ah *AlertHandlers) GetAlert(c echo.Context) error {
// 	id := c.Param("id")
	
// 	alert, found := ah.alertService.GetAlert(id)
// 	if !found {
// 		return c.JSON(http.StatusNotFound, map[string]string{"error": "alert not found"})
// 	}

// 	return c.JSON(http.StatusOK, alert)
// }

// // UpdateAlert godoc
// func (ah *AlertHandlers) UpdateAlert(c echo.Context) error {
// 	id := c.Param("id")
	
// 	var alert models.Alert
// 	if err := c.Bind(&alert); err != nil {
// 		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request body"})
// 	}

// 	if err := ah.alertService.UpdateAlert(id, &alert); err != nil {
// 		return c.JSON(http.StatusNotFound, map[string]string{"error": err.Error()})
// 	}

// 	return c.JSON(http.StatusOK, alert)
// }

// // DeleteAlert godoc
// func (ah *AlertHandlers) DeleteAlert(c echo.Context) error {
// 	id := c.Param("id")
	
// 	if err := ah.alertService.DeleteAlert(id); err != nil {
// 		return c.JSON(http.StatusNotFound, map[string]string{"error": err.Error()})
// 	}

// 	return c.JSON(http.StatusOK, map[string]string{"message": "alert deleted"})
// }

// // GetAlertHistory godoc
// func (ah *AlertHandlers) GetAlertHistory(c echo.Context) error {
// 	limitStr := c.QueryParam("limit")
// 	limit := 100 // Default
	
// 	if limitStr != "" {
// 		if l, err := strconv.Atoi(limitStr); err == nil {
// 			limit = l
// 		}
// 	}

// 	history := ah.alertService.GetHistory(limit)
// 	return c.JSON(http.StatusOK, history)
// }

// // TestAlert godoc - Test an alert without saving
// func (ah *AlertHandlers) TestAlert(c echo.Context) error {
// 	var alert models.Alert
// 	if err := c.Bind(&alert); err != nil {
// 		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request body"})
// 	}

// 	// Create temporary alert for testing
// 	alert.ID = "test_alert"
// 	alert.Enabled = true

// 	// Manually trigger evaluation (simplified)
// 	result := map[string]interface{}{
// 		"alert":   alert,
// 		"message": "Alert test triggered successfully. Check your configured actions.",
// 	}

// 	return c.JSON(http.StatusOK, result)
// }


package handlers

import (
	"net/http"
	"strconv"

	"github.com/labstack/echo/v4"

	"xand/models"
	"xand/services"
)

// AlertHandlers manages alert-related endpoints
type AlertHandlers struct {
	alertService *services.AlertService
}

func NewAlertHandlers(alertService *services.AlertService) *AlertHandlers {
	return &AlertHandlers{
		alertService: alertService,
	}
}

// CreateAlert godoc
func (ah *AlertHandlers) CreateAlert(c echo.Context) error {
	var alert models.Alert
	if err := c.Bind(&alert); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request body"})
	}

	// Validate that only webhook or discord actions are allowed
	if err := ah.validateActions(&alert); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}

	if err := ah.alertService.CreateAlert(&alert); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}

	return c.JSON(http.StatusCreated, alert)
}

// ListAlerts godoc
func (ah *AlertHandlers) ListAlerts(c echo.Context) error {
	alerts := ah.alertService.ListAlerts()
	return c.JSON(http.StatusOK, alerts)
}

// GetAlert godoc
func (ah *AlertHandlers) GetAlert(c echo.Context) error {
	id := c.Param("id")
	
	alert, found := ah.alertService.GetAlert(id)
	if !found {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "alert not found"})
	}

	return c.JSON(http.StatusOK, alert)
}

// UpdateAlert godoc
func (ah *AlertHandlers) UpdateAlert(c echo.Context) error {
	id := c.Param("id")
	
	var alert models.Alert
	if err := c.Bind(&alert); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request body"})
	}

	// Validate that only webhook or discord actions are allowed
	if err := ah.validateActions(&alert); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}

	if err := ah.alertService.UpdateAlert(id, &alert); err != nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": err.Error()})
	}

	return c.JSON(http.StatusOK, alert)
}

// DeleteAlert godoc
func (ah *AlertHandlers) DeleteAlert(c echo.Context) error {
	id := c.Param("id")
	
	if err := ah.alertService.DeleteAlert(id); err != nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": err.Error()})
	}

	return c.JSON(http.StatusOK, map[string]string{"message": "alert deleted"})
}

// GetAlertHistory godoc
func (ah *AlertHandlers) GetAlertHistory(c echo.Context) error {
	limitStr := c.QueryParam("limit")
	limit := 100 // Default
	
	if limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil {
			limit = l
		}
	}

	history := ah.alertService.GetHistory(limit)
	return c.JSON(http.StatusOK, history)
}

// TestAlert godoc - Test an alert without saving
func (ah *AlertHandlers) TestAlert(c echo.Context) error {
	var alert models.Alert
	if err := c.Bind(&alert); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request body"})
	}

	// Validate that only webhook or discord actions are allowed
	if err := ah.validateActions(&alert); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}

	// Create temporary alert for testing
	alert.ID = "test_alert"
	alert.Enabled = true

	// Test the alert
	if err := ah.alertService.TestAlert(&alert); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "Failed to send test alert: " + err.Error(),
		})
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"message": "Test alert sent successfully. Check your configured actions.",
		"alert":   alert,
	})
}

// validateActions ensures only webhook or discord action types are used
func (ah *AlertHandlers) validateActions(alert *models.Alert) error {
	if len(alert.Actions) == 0 {
		return nil // No actions is valid
	}

	for _, action := range alert.Actions {
		switch action.Type {
		case "webhook":
			// Validate webhook has URL
			if url, ok := action.Config["url"].(string); !ok || url == "" {
				return echo.NewHTTPError(http.StatusBadRequest, "webhook action requires 'url' in config")
			}
		case "discord":
			// Discord bot is configured globally, no additional config needed
			// But we can validate if there are any discord-specific settings if needed
			continue
		// case "telegram":
		// 	// Telegram support - currently commented out
		// 	continue
		default:
			return echo.NewHTTPError(http.StatusBadRequest, 
				"invalid action type '"+action.Type+"'. Only 'webhook' and 'discord' are supported")
		}
	}

	return nil
}