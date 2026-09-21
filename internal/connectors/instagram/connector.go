package instagram

import "forge-backend/internal/connectors"

func init() { connectors.Register(&Connector{}) }

type Connector struct{}

func (c *Connector) ID() string                    { return "instagram" }
func (c *Connector) Name() string                  { return "Instagram" }
func (c *Connector) Icon() string                  { return "📸" }
func (c *Connector) Description() string           { return "Visual posts, reels, and stories." }
func (c *Connector) Category() connectors.Category { return connectors.CategoryVisual }
func (c *Connector) Capabilities() connectors.Capabilities {
	return connectors.Capabilities{
		Connect:   connectors.StatusComingSoon,
		Publish:   connectors.StatusPlanned,
		Schedule:  connectors.StatusPlanned,
		Analytics: connectors.StatusPlanned,
	}
}
