package facebook

import "forge-backend/internal/connectors"

func init() { connectors.Register(&Connector{}) }

type Connector struct{}

func (c *Connector) ID() string                    { return "facebook" }
func (c *Connector) Name() string                  { return "Facebook" }
func (c *Connector) Icon() string                  { return "👍" }
func (c *Connector) Description() string           { return "Pages, posts, and community content." }
func (c *Connector) Category() connectors.Category { return connectors.CategoryProfessional }
func (c *Connector) Capabilities() connectors.Capabilities {
	return connectors.Capabilities{
		Connect:   connectors.StatusComingSoon,
		Publish:   connectors.StatusPlanned,
		Schedule:  connectors.StatusPlanned,
		Analytics: connectors.StatusPlanned,
	}
}
