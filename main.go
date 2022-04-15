package main

import (
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/alecthomas/kong"
	"github.com/slack-go/slack"
	"gopkg.in/yaml.v2"
)

const (
	version = "1.0.0"
)

type errInvalidConfig struct{}

func (e errInvalidConfig) Error() string {
	return "Invalid Config File"
}

// cli is the struct used for kong to parse cli args.
var cli struct {
	YmlPath string `arg:"" required:"" help:"The input settings file." type:"path"`
}

type config struct {
	Token string   `yaml:"apitoken,omitempty"`
	Convs []string `yaml:"conversation,omitempty"`
	Users []string `yaml:"userid,omitempty"`
}

// start is the main entry point to the program. p is the path to the yaml file.
func start(p string) error {

	config, err := readYmlFile(p)
	if err != nil {
		return err
	}

	api := slack.New(config.Token)

	convs, err := getConvos(api, config)
	if err != nil {
		return err
	}

	for _, c := range convs {

		err = deleteConvo(api, c)
		if err != nil {
			return err
		}
	}

	return nil
}

// getConvos returns a list of conversation ID, that each are the conversation
// between the bot and the user IDs.
func getConvos(api *slack.Client, config *config) ([]string, error) {

	var convs []string

	if len(config.Convs) == 0 {

		for _, u := range config.Users {

			conversation, err := getConvoFromUser(api, u)
			if err != nil {
				return nil, err
			}

			convs = append(convs, conversation)
		}
	}

	return convs, nil
}

// deleteConvo will delete the all conversation history.
func deleteConvo(api *slack.Client, conv string) error {
	params := slack.GetConversationHistoryParameters{
		ChannelID: conv,
	}
	cont := false
	for !cont {
		hist, err := api.GetConversationHistory(&params)
		if err != nil {
			return err
		}
		if len(hist.Messages) == 0 {
			log.Printf("All messages cleared for channel: %s", conv)
			break
		}
		for _, m := range hist.Messages {
			log.Printf("Deleting message in channel %s with timestamp %s", conv, m.Timestamp)
			_, _, err = api.DeleteMessage(conv, m.Timestamp)
			if err != nil {
				if strings.Contains(err.Error(), "slack rate limit exceeded") {
					seconds := 30
					log.Printf("Slack limit exceeded, sleeping for %d seconds", seconds)
					time.Sleep(time.Duration(seconds) * time.Second)
				} else {
					return err
				}
			}
		}
		cont = hist.HasMore
	}
	return nil
}

func getConvoFromUser(api *slack.Client, user string) (string, error) {
	conv, err := getChannelIDFromUser(user, api)
	if err != nil {
		return "", err
	}
	return conv, nil
}

// getChannelIDFromUser will open a DM with the provided userID string, and return the channel
// ID so it can be used for sending messages.
func getChannelIDFromUser(userID string, api *slack.Client) (string, error) {
	params := slack.OpenConversationParameters{
		Users: []string{userID},
	}
	channel, _, _, err := api.OpenConversation(&params)
	if err != nil {
		return "", err
	}
	return channel.ID, nil
}

func readYmlFile(p string) (*config, error) {
	b, err := os.ReadFile(p)
	if err != nil {
		return nil, err
	}
	var c config
	err = yaml.Unmarshal(b, &c)
	if err != nil {
		return nil, err
	}
	return validateYmlFile(&c)
}

// validateYmlFile will validate the config.
func validateYmlFile(c *config) (*config, error) {
	if c.Token == "" {
		return nil, fmt.Errorf("invalid api token")
	}
	if len(c.Users) == 0 && len(c.Convs) == 0 {
		return nil, fmt.Errorf("Need either one user or conversation")
	}
	return c, nil
}

func main() {
	kong.Parse(&cli,
		kong.Name("Slack dm cleaner"),
		kong.Description("An easy button to clear DMs when using a slack app"),
		kong.UsageOnError(),
		kong.ConfigureHelp(kong.HelpOptions{
			Compact: true,
		}),
		kong.Vars{
			"version": version,
		},
	)
	err := start(cli.YmlPath)
	if err != nil {
		log.Printf("Starting slack cleaner: %s", err)
	}
}
