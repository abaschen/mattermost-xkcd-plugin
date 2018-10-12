package main

import (
	"bytes"
	"encoding/gob"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"sync"

	"github.com/mattermost/mattermost-server/model"
	"github.com/mattermost/mattermost-server/plugin"
	"github.com/pkg/errors"
)

type XKCD struct {
	Day   string `json:"day"`
	Month string `json:"month"`
	Year  string `json:"year"`

	Num       int    `json:"num"`
	Link      string `json:"link"`
	SafeTitle string `json:"safe_title"`
	Img       string `json:"img"`

	News       string `json:"transcript "`
	Transcript string `json:"transcript"`
	Alt        string `json:"alt"`
	Title      string `json:"title"`
}
type XKCDPlugin struct {
	plugin.MattermostPlugin
	Client *http.Client
	// configurationLock synchronizes access to the configuration.
	configurationLock sync.RWMutex

	// configuration is the active plugin configuration. Consult getConfiguration and
	// setConfiguration for usage.
	configuration *configuration
}

type configuration struct {
	Debug         bool
	StrictTrigger bool
}

// Clone deep copies the configuration
func (c *configuration) Clone() *configuration {
	return &configuration{
		Debug:         c.Debug,
		StrictTrigger: c.StrictTrigger,
	}
}

// getConfiguration retrieves the active configuration under lock, making it safe to use
// concurrently. The active configuration may change underneath the client of this method, but
// the struct returned by this API call is considered immutable.
func (p *XKCDPlugin) getConfiguration() *configuration {
	p.configurationLock.RLock()
	defer p.configurationLock.RUnlock()

	if p.configuration == nil {
		return &configuration{}
	}

	return p.configuration
}

// setConfiguration replaces the active configuration under lock.
func (p *XKCDPlugin) setConfiguration(configuration *configuration) {
	p.configurationLock.Lock()
	defer p.configurationLock.Unlock()

	if configuration != nil && p.configuration == configuration {
		panic("setConfiguration called with the existing configuration")
	}

	p.configuration = configuration
}

// OnConfigurationChange is invoked when configuration changes may have been made.
func (p *XKCDPlugin) OnConfigurationChange() error {
	var configuration = new(configuration)

	// Load the public configuration fields from the Mattermost server configuration.
	if err := p.API.LoadPluginConfiguration(configuration); err != nil {
		return errors.Wrap(err, "failed to load plugin configuration")
	}

	p.setConfiguration(configuration)

	return nil
}

/**
MessageWillBePosted is invoked when a message is posted by a user before it is committed to the database. If you also want to act on edited posts, see MessageWillBeUpdated.

To reject a post, return an non-empty string describing why the post was rejected. To modify the post, return the replacement, non-nil *model.Post and an empty string. To allow the post without modification, return a nil *model.Post and an empty string.

If you don't need to modify or reject posts, use MessageHasBeenPosted instead.

Note that this method will be called for posts created by plugins, including the plugin that created the post.
*/
func (o *XKCDPlugin) MessageWillBePosted(c *plugin.Context, post *model.Post) (*model.Post, string) {
	message := post.Message
	config := o.getConfiguration()
	if o.Client == nil {
		o.Client = http.DefaultClient
	}
	xkcd := UpdatePost(message, o)
	if xkcd == nil {
		return nil, ""
	}
	gob.Register(model.SlackAttachment{})
	//post.Message = "[![](" + src + " \"" + title + "\")](" + url + ")"
	post.AddProp("xkcd", true)
	attachments := post.Attachments()
	var nonNilAttachments []*model.SlackAttachment
	attachment := model.SlackAttachment{
		Title: "XKCD Comic - " + xkcd.SafeTitle,

		TitleLink: "https://xkcd.com/" + strconv.Itoa(xkcd.Num) + "/",
		ImageURL:  xkcd.Img,
		Text:      xkcd.Alt,
	}
	//attachments = append(nonNilAttachments, *attachments)
	for _, a := range attachments {
		if a == nil {
			continue
		}
		nonNilAttachments = append(nonNilAttachments, a)
	}
	nonNilAttachments = append(nonNilAttachments, &attachment)

	post.AddProp("attachments", nonNilAttachments)
	if config.Debug {
		fmt.Println("Post modified with the attachment :)")
	}
	return post, ""
}

func UpdatePost(message string, o *XKCDPlugin) *XKCD {
	config := o.getConfiguration()
	debug := config.Debug
	if debug {
		fmt.Println("Message received - processing")
	}
	var re *regexp.Regexp
	if config.StrictTrigger {
		re = regexp.MustCompile("^(http(s?):\\/\\/)?xkcd\\.com\\/(\\d+)(\\/?)$")
	} else {
		re = regexp.MustCompile("(http(s?):\\/\\/)?xkcd\\.com\\/(\\d+)(\\/?)")
	}
	url := re.FindString(message)

	if url == "" {
		if debug {
			fmt.Println("No URL found - skipping")
		}
		return nil
	}
	num := re.FindStringSubmatch(url)[3]
	if debug {
		fmt.Println("URL found - Fetching info for comic " + num)
	}
	url = "https://xkcd.com/" + num + "/info.0.json"
	resp, err := o.Client.Get(url)

	if err != nil {
		if debug {
			fmt.Println("ERROR: Failed to get JSON info \"" + url + "\" " + err.Error())
		}
		return nil
	}
	xkcd := XKCD{}

	b := resp.Body
	defer b.Close() // close Body when the function returns
	buf := new(bytes.Buffer)
	buf.ReadFrom(b)
	if debug {
		fmt.Println("Received JSON info \"" + buf.String() + "\"")
	}
	err = json.Unmarshal(buf.Bytes(), &xkcd)

	if err != nil {
		// passthrough
		if debug {
			fmt.Println("Error unmarshalling JSON")
		}
		return nil
	}
	if debug {
		fmt.Println("XKCD comic extracted  \"" + num + "\": \"" + xkcd.Img + "\"")
	}

	return &xkcd
}

func main() {
	plugin.ClientMain(&XKCDPlugin{})
}
