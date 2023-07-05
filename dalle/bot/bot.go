package bot

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math"
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

type Bot struct {
	gcpProjectID    string
	apiKeyGCPSecret string
	openaiClient    *client.Client
}

var (
	integerOptionMinValue = 1.0

	Commands = []*discordgo.ApplicationCommand{
		{
			Name:        "version",
			Description: "Get the version information for this bot",
		},
		{
			Name:        "generate",
			Description: "Creates an image given a prompt",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Name:        "image-size",
					Description: "Size of the image (S/M/L)",
					Type:        discordgo.ApplicationCommandOptionString,
					Choices: []*discordgo.ApplicationCommandOptionChoice{
						{
							Name:  "Small",
							Value: "small",
						},
						{
							Name:  "Medium",
							Value: "medium",
						},
						{
							Name:  "Large",
							Value: "large",
						},
					},
					Required: true,
				},
				{
					Name:        "image-count",
					Description: "Number of images to generate",
					Type:        discordgo.ApplicationCommandOptionInteger,
					Required:    true,
					MinValue:    &integerOptionMinValue,
					MaxValue:    10,
				},
				{
					Name:        "image-prompt",
					Description: "Text prompt to pass to Dall-e 2",
					Type:        discordgo.ApplicationCommandOptionString,
					Required:    true,
				},
			},
		},
		{
			Name:        "generate-single-large",
			Description: "Creates a single largeimage given a prompt",
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

func (b *Bot) Version(c *webhooks.Client, i *discordgo.Interaction, r *http.Request) error {
	version := fmt.Sprintf("Dall-e Bot Version: %s", version.Version)
	return c.SendStringResponse(i, version)
}

func (b *Bot) Generate(c *webhooks.Client, i *discordgo.Interaction, r *http.Request) error {
	imageSize := i.ApplicationCommandData().Options[0].Value.(string)
	imageCount := int(math.Round(i.ApplicationCommandData().Options[1].Value.(float64)))
	imagePrompt := i.ApplicationCommandData().Options[2].Value.(string)

	if err := b.initOpenAIClient(); err != nil {
		return fmt.Errorf("b.initOpenAIClient: %v", err)
	}

	req := &image.CreateRequest{
		Prompt: imagePrompt,
		Size:   imageSize,
		N:      imageCount,
	}

	data, err := b.openaiClient.Image.Create(context.Background(), req)
	if err != nil {
		return c.SendStringResponse(i, fmt.Sprintf("b.openaiClient.Image.Create: %v", err))
	}

	pngs, err := b.createPngs(data)
	if err != nil {
		return c.SendStringResponse(i, fmt.Sprintf("createPngFiles: %v", err))
	}

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
