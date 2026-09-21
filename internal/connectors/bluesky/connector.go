package bluesky

import "forge-backend/internal/connectors"

func init() { connectors.Register(&Connector{}) }

type Connector struct{}

func (c *Connector) ID() string                    { return "bluesky" }
func (c *Connector) Name() string                  { return "Bluesky" }
func (c *Connector) Icon() string                  { return "🦋" }
func (c *Connector) Description() string           { return "Decentralized short-form posts." }
func (c *Connector) Category() connectors.Category { return connectors.CategorySocial }
func (c *Connector) Capabilities() connectors.Capabilities {
	return connectors.Capabilities{
		Connect:   connectors.StatusComingSoon,
		Publish:   connectors.StatusPlanned,
		Schedule:  connectors.StatusPlanned,
		Analytics: connectors.StatusPlanned,
	}
}
