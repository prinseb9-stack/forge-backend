package mastodon

import "forge-backend/internal/connectors"

func init() { connectors.Register(&Connector{}) }

type Connector struct{}

func (c *Connector) ID() string                    { return "mastodon" }
func (c *Connector) Name() string                  { return "Mastodon" }
func (c *Connector) Icon() string                  { return "🐘" }
func (c *Connector) Description() string           { return "Federated social posts." }
func (c *Connector) Category() connectors.Category { return connectors.CategorySocial }
func (c *Connector) Capabilities() connectors.Capabilities {
	return connectors.Capabilities{
		Connect:   connectors.StatusComingSoon,
		Publish:   connectors.StatusPlanned,
		Schedule:  connectors.StatusPlanned,
		Analytics: connectors.StatusPlanned,
	}
}
