package telegram

import "forge-backend/internal/connectors"

func init() { connectors.Register(&Connector{}) }

type Connector struct{}

func (c *Connector) ID() string                    { return "telegram" }
func (c *Connector) Name() string                  { return "Telegram" }
func (c *Connector) Icon() string                  { return "✈️" }
func (c *Connector) Description() string           { return "Channels and group messaging." }
func (c *Connector) Category() connectors.Category { return connectors.CategoryMessaging }
func (c *Connector) Capabilities() connectors.Capabilities {
	return connectors.Capabilities{
		Connect:   connectors.StatusComingSoon,
		Publish:   connectors.StatusPlanned,
		Schedule:  connectors.StatusPlanned,
		Analytics: connectors.StatusPlanned,
	}
}
