package medium

import "forge-backend/internal/connectors"

func init() { connectors.Register(&Connector{}) }

type Connector struct{}

func (c *Connector) ID() string                    { return "medium" }
func (c *Connector) Name() string                  { return "Medium" }
func (c *Connector) Icon() string                  { return "✍️" }
func (c *Connector) Description() string           { return "Long-form article publishing." }
func (c *Connector) Category() connectors.Category { return connectors.CategoryPublishing }
func (c *Connector) Capabilities() connectors.Capabilities {
	return connectors.Capabilities{
		Connect:   connectors.StatusComingSoon,
		Publish:   connectors.StatusPlanned,
		Schedule:  connectors.StatusPlanned,
		Analytics: connectors.StatusPlanned,
	}
}
