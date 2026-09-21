package wordpress

import "forge-backend/internal/connectors"

func init() { connectors.Register(&Connector{}) }

type Connector struct{}

func (c *Connector) ID() string                    { return "wordpress" }
func (c *Connector) Name() string                  { return "WordPress" }
func (c *Connector) Icon() string                  { return "🌐" }
func (c *Connector) Description() string           { return "Self-hosted and hosted blog publishing." }
func (c *Connector) Category() connectors.Category { return connectors.CategoryPublishing }
func (c *Connector) Capabilities() connectors.Capabilities {
	return connectors.Capabilities{
		Connect:   connectors.StatusComingSoon,
		Publish:   connectors.StatusPlanned,
		Schedule:  connectors.StatusPlanned,
		Analytics: connectors.StatusPlanned,
	}
}
