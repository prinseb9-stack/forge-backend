package pinterest

import "forge-backend/internal/connectors"

func init() { connectors.Register(&Connector{}) }

type Connector struct{}

func (c *Connector) ID() string                    { return "pinterest" }
func (c *Connector) Name() string                  { return "Pinterest" }
func (c *Connector) Icon() string                  { return "📌" }
func (c *Connector) Description() string           { return "Pins and visual discovery." }
func (c *Connector) Category() connectors.Category { return connectors.CategoryVisual }
func (c *Connector) Capabilities() connectors.Capabilities {
	return connectors.Capabilities{
		Connect:   connectors.StatusComingSoon,
		Publish:   connectors.StatusPlanned,
		Schedule:  connectors.StatusPlanned,
		Analytics: connectors.StatusPlanned,
	}
}
