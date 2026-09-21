// Package all blank-imports every connector package so their init()
// functions run and register each connector with the global registry.
//
// Add one import line here for each new connector. Nothing else needs
// to change anywhere in FORGE when a new platform is added.
package all

import (
	_ "forge-backend/internal/connectors/blog"
	_ "forge-backend/internal/connectors/bluesky"
	_ "forge-backend/internal/connectors/discord"
	_ "forge-backend/internal/connectors/facebook"
	_ "forge-backend/internal/connectors/ghost"
	_ "forge-backend/internal/connectors/google_business"
	_ "forge-backend/internal/connectors/instagram"
	_ "forge-backend/internal/connectors/linkedin"
	_ "forge-backend/internal/connectors/mastodon"
	_ "forge-backend/internal/connectors/medium"
	_ "forge-backend/internal/connectors/newsletter"
	_ "forge-backend/internal/connectors/pinterest"
	_ "forge-backend/internal/connectors/reddit"
	_ "forge-backend/internal/connectors/snapchat"
	_ "forge-backend/internal/connectors/substack"
	_ "forge-backend/internal/connectors/telegram"
	_ "forge-backend/internal/connectors/threads"
	_ "forge-backend/internal/connectors/tiktok"
	_ "forge-backend/internal/connectors/tumblr"
	_ "forge-backend/internal/connectors/twitch"
	_ "forge-backend/internal/connectors/whatsapp"
	_ "forge-backend/internal/connectors/wordpress"
	_ "forge-backend/internal/connectors/x"
	_ "forge-backend/internal/connectors/youtube_shorts"
)
