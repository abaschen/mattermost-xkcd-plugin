package main

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/mattermost/mattermost-server/model"
	"github.com/mattermost/mattermost-server/plugin"
	"github.com/mattermost/mattermost-server/plugin/plugintest"
	"github.com/mattermost/mattermost-server/plugin/plugintest/mock"
)

type TestConfiguration struct {
	StrictTrigger bool
	Debug         bool
	Client        *http.Client
}

func TestPlugin(t *testing.T) {
	jsonComic := `{"month": "10", "num": 2057, "link": "", "year": "2018", "news": "", "safe_title": "Internal Monologues", "transcript": "", "alt": "Haha, just kidding, everyone's already been hacked. I wonder if today's the day we find out about it.", "img": "https://imgs.xkcd.com/comics/internal_monologues.png", "title": "Internal Monologues", "day": "10"}`
	ts := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, jsonComic)
	}))
	defer ts.Close()
	client := ts.Client()

	validConfiguration := TestConfiguration{
		StrictTrigger: false,
		Debug:         true,
		Client:        client,
	}
	validConfigurationStrict := TestConfiguration{
		StrictTrigger: true,
		Debug:         true,
		Client:        client,
	}

	for name, tc := range map[string]struct {
		Configuration    TestConfiguration
		Message          string
		ExpectedComicNum int
		ExpectedComic    bool
	}{
		"NoConfiguration": {
			Configuration:    validConfiguration,
			Message:          "sdfsdf https://xkcd.com/2057/ sdfsdf",
			ExpectedComicNum: 2057,
			ExpectedComic:    true,
		},
		"BadUrl": {
			Configuration: validConfiguration,
			Message:       "sdfsdf https://xdkcd.com/2057/ sdfsdf",
			ExpectedComic: false,
		},
		"GoodUrlStrictSkip": {
			Configuration: validConfigurationStrict,
			Message:       "sdfsdf https://xkcd.com/2057/ sdfsdf",
			ExpectedComic: false,
		},
		"GoodUrlStrict": {
			Configuration:    validConfigurationStrict,
			Message:          "https://xkcd.com/2057/",
			ExpectedComicNum: 2057,
			ExpectedComic:    true,
		},
	} {
		t.Run(name, func(t *testing.T) {
			api := &plugintest.API{}

			api.On("CreatePost", mock.AnythingOfType("*model.Post")).Return(&model.Post{})

			p := XKCDPlugin{}
			p.Debug = tc.Configuration.Debug
			p.StrictTrigger = tc.Configuration.StrictTrigger
			p.SetAPI(api)

			post := model.Post{
				Message: tc.Message,
			}

			result, sErr := p.MessageWillBePosted(&plugin.Context{}, &post)
			assert.Equal(t, sErr, "")

			if tc.ExpectedComic {
				assert.NotNil(t, result)
				assert.Equal(t, tc.ExpectedComic, result.Props["xkcd"] != nil)
				attachments := result.Attachments()
				assert.NotNil(t, attachments)
				for _, a := range attachments {
					if a == nil {
						continue
					}
					assert.Equal(t, "https://xkcd.com/"+strconv.Itoa(tc.ExpectedComicNum)+"/", a.TitleLink)
				}
			} else {
				assert.Nil(t, result)
			}
		})
	}
}
