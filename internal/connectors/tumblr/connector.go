package tumblr

import "forge-backend/internal/connectors"

func init() { connectors.Register(&Connector{}) }

type Connector struct{}

func (c *Connector) ID() string                    { return "tumblr" }
func (c *Connector) Name() string                  { return "Tumblr" }
func (c *Connector) Icon() string                  { return "📓" }
func (c *Connector) Description() string           { return "Blog-style social posts." }
func (c *Connector) Category() connectors.Category { return connectors.CategorySocial }
func (c *Connector) Capabilities() connectors.Capabilities {
	return connectors.Capabilities{
		Connect:   connectors.StatusComingSoon,
		Publish:   connectors.StatusPlanned,
		Schedule:  connectors.StatusPlanned,
		Analytics: connectors.StatusPlanned,
	}
}
