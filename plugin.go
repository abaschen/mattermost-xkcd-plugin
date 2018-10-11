package main

import (
	"fmt"
	"net/http"
	"regexp"
	"strings"

	"github.com/mattermost/mattermost-server/model"
	"github.com/mattermost/mattermost-server/plugin"
	"golang.org/x/net/html"
)

type XKCDPlugin struct {
	plugin.MattermostPlugin
}

func getHtml(url string) (*html.Node, error) {
	resp, err := http.Get(url)

	if err != nil {
		fmt.Println("ERROR: Failed to crawl \"" + url + "\"")
		return nil, err
	}

	b := resp.Body
	defer b.Close() // close Body when the function returns

	return html.Parse(b)
}

/**
MessageWillBePosted is invoked when a message is posted by a user before it is committed to the database. If you also want to act on edited posts, see MessageWillBeUpdated.

To reject a post, return an non-empty string describing why the post was rejected. To modify the post, return the replacement, non-nil *model.Post and an empty string. To allow the post without modification, return a nil *model.Post and an empty string.

If you don't need to modify or reject posts, use MessageHasBeenPosted instead.

Note that this method will be called for posts created by plugins, including the plugin that created the post.
*/
func (o *XKCDPlugin) MessageWillBePosted(c *plugin.Context, post *model.Post) (*model.Post, string) {

	re := regexp.MustCompile("^(http(s?):\\/\\/)?xkcd\\.com\\/(\\d+)(\\/?)$")
	url := re.FindString(post.Message)
	if post.Attachments() != nil || url == "" {
		return nil, ""
	}

	doc, err := getHtml(url)

	if err != nil {
		// passthrough
		return nil, ""
	}
	comic := getElementById(doc, "comic")
	for c := comic.FirstChild; c != nil; c = c.NextSibling {
		if c.Data == "img" {
			ok, src, title := getImg(c)
			if ok {
				post.Message = "[![](" + src + " \"" + title + "\")](" + url + ")"
				post.AddProp("xkcd", true)
				return post, ""
			} else {
				return nil, ""
			}
		}
	}
	return nil, ""
}

////////// GET ELEMENT BY ID helper
func GetId(n *html.Node) (string, bool) {
	for _, attr := range n.Attr {
		if attr.Key == "id" {
			return attr.Val, true
		}
	}
	return "", false
}

func checkId(n *html.Node, id string) bool {
	if n.Type == html.ElementNode {
		s, ok := GetId(n)
		if ok && s == id {
			return true
		}
	}
	return false
}

func traverse(n *html.Node, id string) *html.Node {
	if checkId(n, id) {
		return n
	}

	for c := n.FirstChild; c != nil; c = c.NextSibling {
		result := traverse(c, id)
		if result != nil {
			return result
		}
	}

	return nil
}

func getElementById(n *html.Node, id string) *html.Node {
	return traverse(n, id)
}

////////////////// end

// Helper function to pull the href attribute from a Token
func getImg(n *html.Node) (ok bool, src string, title string) {
	title = "From xkcd.com"
	// Iterate over all of the Token's attributes until we find an "href"
	for _, a := range n.Attr {
		if a.Key == "src" {
			src = a.Val
			ok = true
		} else if a.Key == "title" {
			title = strings.Replace(a.Val, "\"", "\\\"", -1)
		}
	}

	// "bare" return will return the variables (ok, href) as defined in
	// the function definition
	return ok, src, title
}

func main() {
	plugin.ClientMain(&XKCDPlugin{})
}
