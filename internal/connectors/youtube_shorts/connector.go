package youtube_shorts

import "forge-backend/internal/connectors"

func init() { connectors.Register(&Connector{}) }

type Connector struct{}

func (c *Connector) ID() string                    { return "youtube-shorts" }
func (c *Connector) Name() string                  { return "YouTube Shorts" }
func (c *Connector) Icon() string                  { return "🎬" }
func (c *Connector) Description() string           { return "Short-form video content." }
func (c *Connector) Category() connectors.Category { return connectors.CategoryVideo }
func (c *Connector) Capabilities() connectors.Capabilities {
	return connectors.Capabilities{
		Connect:   connectors.StatusComingSoon,
		Publish:   connectors.StatusPlanned,
		Schedule:  connectors.StatusPlanned,
		Analytics: connectors.StatusPlanned,
	}
}
