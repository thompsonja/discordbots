package bot

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/thompsonja/discord_bots_lib/pkg/discord/webhooks"
	"github.com/thompsonja/discord_bots_lib/pkg/gcp/secrets"
	"github.com/thompsonja/discord_bots_lib/pkg/version"
	"github.com/thompsonja/openai-go/pkg/client"
	"github.com/thompsonja/openai-go/pkg/image"
)

// Define the structure of our Dalle bot
type Bot struct {
	gcpProjectID    string
	apiKeyGCPSecret string
	openaiClient    *client.Client
}

var (
	Commands = []*discordgo.ApplicationCommand{
		{
			Name:        "debug",
			Description: "Debugging command that returns an error",
		},
		{
			Name:        "version",
			Description: "Get the version information for this bot",
		},
		{
			Name:        "generate",
			Description: "Creates a single large image given a prompt",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Name:        "image-prompt",
					Description: "Text prompt to pass to Dall-e 2",
					Type:        discordgo.ApplicationCommandOptionString,
					Required:    true,
				},
			},
		},
	}
)

func New(gcpProjectID, openaiApiSecret string) *Bot {
	return &Bot{
		gcpProjectID:    gcpProjectID,
		apiKeyGCPSecret: openaiApiSecret,
	}
}

func (b *Bot) initOpenAIClient() error {
	if b.openaiClient != nil {
		return nil
	}

	apiKey, err := secrets.GetLatestSecretValue(b.apiKeyGCPSecret, b.gcpProjectID)
	if err != nil {
		return fmt.Errorf("initOpenAIClient: %v", err)
	}
	b.openaiClient = client.New(strings.TrimSpace(apiKey))
	return nil
}

// Debug returns an error to test email notifications
func (b *Bot) Debug(c *webhooks.Client, i *discordgo.Interaction, r *http.Request) error {
	c.SendStringResponse(i, "Returned an error")
	return fmt.Errorf("an example error")
}

// Version returns the git commit sha of the bot
func (b *Bot) Version(c *webhooks.Client, i *discordgo.Interaction, r *http.Request) error {
	version := fmt.Sprintf("Dall-e Bot Version: %s", version.Version)
	return c.SendStringResponse(i, version)
}

// Generate creates a single large image given a prompt
func (b *Bot) Generate(c *webhooks.Client, i *discordgo.Interaction, r *http.Request) error {
	imagePrompt := i.ApplicationCommandData().Options[0].Value.(string)

	// Initialize the OpenAI client using your API key
	if err := b.initOpenAIClient(); err != nil {
		c.SendStringResponse(i, fmt.Sprintf("Failed to initialize OpenAI client: %v", err))
		return err
	}

	// Send the prompt to the OpenAI API
	data, err := b.openaiClient.Image.Create(context.Background(), &image.CreateRequest{
		Prompt: imagePrompt,
		Size:   "large",
		N:      1,
	})
	if err != nil {
		return c.SendStringResponse(i, fmt.Sprintf("Failed to create image: %v", err))
	}

	// Conver the byte data response to pngs
	pngs, err := b.createPngs(data)
	if err != nil {
		c.SendStringResponse(i, fmt.Sprintf("Failed to create png files: %v", err))
		return err
	}

	// Create Discord file objects and send them as a response
	files := []*discordgo.File{}
	now := time.Now().Format("20060102_150405")
	for idx, png := range pngs {
		files = append(files, &discordgo.File{
			Name:        fmt.Sprintf("%s_%d.png", now, idx),
			ContentType: "image/png",
			Reader:      bytes.NewBuffer(png),
		})
	}
	return c.SendFilesResponse(i, files)
}

type dalleDataResponse struct {
	URL      string `json:"url,omitempty"`
	B64_json string `json:"b64_json,omitempty"`
}

type dalleResponse struct {
	Created int                  `json:"created,omitempty"`
	Data    []*dalleDataResponse `json:"data,omitempty"`
}

func (b *Bot) createPngs(data []byte) ([][]byte, error) {
	var d dalleResponse
	if err := json.Unmarshal(data, &d); err != nil {
		return nil, fmt.Errorf("json.Unmarshal: %v", err)
	}

	ret := [][]byte{}
	for _, dataResponse := range d.Data {
		pngData, err := base64.StdEncoding.DecodeString(dataResponse.B64_json)
		if err != nil {
			return nil, fmt.Errorf("base64.StdEncoding.DecodeString: %v", err)
		}
		ret = append(ret, pngData)
	}

	return ret, nil
}
