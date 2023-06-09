package main

import (
	"context"
	"crypto/ed25519"
	"encoding/hex"
	"flag"
	"log"
	"os/exec"
	"strings"

	"cloud.google.com/go/logging"
	"github.com/thompsonja/discord_bots_lib/pkg/discord/webhooks"
	gcplogger "github.com/thompsonja/discord_bots_lib/pkg/gcp/logger"
	"github.com/thompsonja/discord_bots_lib/pkg/logger"
	"github.com/thompsonja/discordbots/dalle/bot"
)

// from https://discord.com/developers/applications/<app_id>/information
const (
	appID     = "1096915795826712636"
	publicKey = "a5b67d4ad09f4df5623212b62fe8eefbb9c2c0758885d3d95d16ab312a3062bd"
)

var (
	pk, _ = hex.DecodeString(publicKey)
	epk   = ed25519.PublicKey(pk)
)

func main() {
	var gcpProjectID = flag.String("project_id", "", "GCP Project ID")
	var port = flag.Int("port", 8080, "server port")
	var local = flag.Bool("local", false, "Run locally")
	var updateCommands = flag.Bool("update", false, "Update commands")
	var destroyCommands = flag.Bool("destroy", false, "Destroy commands")
	var openaiApiSecret = flag.String("openai_api_secret", "openai-api-secret", "GCP secret with openai-api key")

	flag.Parse()

	// Get project ID from your gcloud config if not passed in as a flag.
	if *gcpProjectID == "" {
		cmd := exec.Command("gcloud", "config", "get-value", "project")
		stdoutStderr, err := cmd.CombinedOutput()
		if err != nil {
			log.Fatalf("cmd.CombinedOutput: %v", err)
		}
		*gcpProjectID = strings.TrimSpace(string(stdoutStderr))
	}

	// Instantiate a bot.
	b := bot.New(*gcpProjectID, *openaiApiSecret)

	// Map Discord Application Commands to bot functions.
	fns := map[string]webhooks.WebhookFunc{
		"debug":    b.Debug,
		"version":  b.Version,
		"generate": b.Generate,
	}

	var l logger.Logger
	if *local {
		l = &logger.StandardLogger{}
	} else {
		// Create a GCP logging client
		client, err := logging.NewClient(context.Background(), *gcpProjectID)
		if err != nil {
			log.Fatalf("Failed to create client: %v", err)
		}
		defer client.Close()
		l = gcplogger.New(client, "dalle-logs")
	}

	// Create a new webhook client.
	// SecretKey is the name of the GCP Secret that was automatically created by
	// the terraform configs and manually populated.
	c, err := webhooks.NewClient(webhooks.ClientConfig{
		AppID:     appID,
		Commands:  bot.Commands,
		Port:      *port,
		Epk:       epk,
		Fns:       fns,
		ProjectID: *gcpProjectID,
		SecretKey: "dalle-key",
		Logger:    l,
	})
	if err != nil {
		log.Fatalf("discord.NewClient: %v", err)
	}

	// Run the bot, destroying and updating commands if desired.
	if err := c.Run(*destroyCommands, *updateCommands); err != nil {
		log.Fatalf("c.Run: %v", err)
	}
}
