package google_business

import "forge-backend/internal/connectors"

func init() { connectors.Register(&Connector{}) }

type Connector struct{}

func (c *Connector) ID() string                    { return "google-business" }
func (c *Connector) Name() string                  { return "Google Business Profile" }
func (c *Connector) Icon() string                  { return "🏪" }
func (c *Connector) Description() string           { return "Business updates and local posts." }
func (c *Connector) Category() connectors.Category { return connectors.CategoryProfessional }
func (c *Connector) Capabilities() connectors.Capabilities {
	return connectors.Capabilities{
		Connect:   connectors.StatusComingSoon,
		Publish:   connectors.StatusPlanned,
		Schedule:  connectors.StatusPlanned,
		Analytics: connectors.StatusPlanned,
	}
}
