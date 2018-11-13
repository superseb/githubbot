package main

import (
	"context"
	"io/ioutil"
	"net/http"
	"os"
	"regexp"
	"strings"

	gh "github.com/google/go-github/v18/github"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"
	"golang.org/x/oauth2"
	"gopkg.in/go-playground/webhooks.v5/github"
)

const (
	path = "/webhooks"
)

var (
	VERSION = "v0.0.0-dev"
)

func main() {
	app := cli.NewApp()
	app.Name = "githubbot"
	app.Version = VERSION
	app.Usage = "githubbot"
	app.Flags = []cli.Flag{
		cli.BoolFlag{
			Name:  "debug",
			Usage: "Enable debug logs",
		},
		cli.StringFlag{
			Name:   "webhooktoken-file",
			EnvVar: "WEBHOOK_TOKEN_FILE",
			Value:  "/etc/githubbot/webhooktoken",
		},
		cli.StringFlag{
			Name:   "patoken-file",
			EnvVar: "PA_TOKEN_FILE",
			Value:  "/etc/githubbot/patoken",
		},
		cli.StringFlag{
			Name:   "github-org",
			EnvVar: "GH_ORG",
		},
		cli.StringFlag{
			Name:   "github-repo",
			EnvVar: "GH_REPO",
		},
	}
	app.Action = run

	if err := app.Run(os.Args); err != nil {
		logrus.Fatal(err)
	}
}

func run(c *cli.Context) error {
	if c.Bool("debug") {
		logrus.SetLevel(logrus.DebugLevel)
		logrus.Debugf("Debug logging set")
	}
	logrus.Infof("Starting githubbot %s", VERSION)
	// Set vars
	versionregexes := map[string]string{
		"rancher/(server|rancher)(:|)(\\s+|\\n+|)+(v|)1": "version/1.6",
		"\\|Versions\\|Rancher\\s\\`v1":                  "version/1.6",
		"Rancher\\sversion.+:(\\s+|\\n+|)(v|)1":          "version/1.6",
		"(rancher/|)rancher(:|)(\\s+|\\n+|)+(v|)2":       "version/2.0",
		"\\|Versions\\|Rancher\\s\\`v2":                  "version/2.0",
		"Rancher\\sversion.+:(\\s+|\\n+|)(v|)2":          "version/2.0",
	}
	kindpattern := `What kind of request is this(?s).*?:(?s).*?(\w+|\w+\s+\w+)\r\n`
	kinds := map[string]string{
		"question":        "kind/question",
		"bug":             "kind/bug",
		"enhancement":     "kind/enhancement",
		"feature request": "kind/feature",
		"feature":         "kind/feature",
	}

	// Setup webhook token (for validating incoming webhooks)
	data, err := ioutil.ReadFile(c.String("webhooktoken-file"))
	if err != nil {
		logrus.Errorf("Could not load webhooktoken-file from %s: %v\n", c.String("webhooktoken-file"), err)
		os.Exit(1)
	}
	webhooktoken := string(data)
	webhooktoken = strings.TrimSuffix(webhooktoken, "\n")

	hook, err := github.New(github.Options.Secret(webhooktoken))
	if err != nil {
		logrus.Error(err)
		os.Exit(1)
	}

	// Setup personal access token (for executing actions on GitHub)
	data, err = ioutil.ReadFile(c.String("patoken-file"))
	if err != nil {
		logrus.Errorf("Could not load patoken-file from %s: %v\n", c.String("patoken-file"), err)
		os.Exit(1)
	}
	patoken := string(data)
	patoken = strings.TrimSuffix(patoken, "\n")

	ctx := context.Background()
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: patoken},
	)
	tc := oauth2.NewClient(ctx, ts)

	client := gh.NewClient(tc)

	http.HandleFunc(path, func(w http.ResponseWriter, r *http.Request) {
		logrus.Debugf("Incoming HTTP request: %v\n", r)
		payload, err := hook.Parse(r, github.IssuesEvent)
		if err != nil {
			if err == github.ErrEventNotFound {
				logrus.Debugf("Not a matched event: %s\n", err)
			} else {
				logrus.Errorf("Error while parsing webhook: %s\n", err)
			}
		}
		switch payload.(type) {
		case github.IssuesPayload:
			issuepayload := payload.(github.IssuesPayload)
			issue := issuepayload.Issue
			body := issue.Body
			logrus.Infof("Incoming payload for issue #%d with action %s", int(issue.Number), issuepayload.Action)
			logrus.Debugf("Payload: %#v\n", issuepayload)
			// Only acting on newly opened issues
			if issuepayload.Action != "opened" {
				break
			}
			if len(issue.Labels) != 0 {
				logrus.Infof("Skipping issue #%d, has labels on creation\n", int(issue.Number))
				break
			}
			logrus.Debugf("Body: %q\n", body)
			var labelsToApply []string
			for regex, label := range versionregexes {
				r := regexp.MustCompile(regex)
				if r.MatchString(body) {
					logrus.Infof("Adding label %s to issue #%d\n", label, int(issue.Number))
					labelsToApply = append(labelsToApply, label)
				}
			}
			re := regexp.MustCompile(kindpattern)
			if err != nil {
				logrus.Error(err)
			}
			results := re.FindStringSubmatch(body)
			if len(results) > 1 {
				kind := strings.ToLower(results[1])
				if val, ok := kinds[kind]; ok {
					logrus.Infof("Adding label %s to issue #%d\n", val, int(issue.Number))
					labelsToApply = append(labelsToApply, val)
				}
			}

			_, _, err = client.Issues.AddLabelsToIssue(ctx, c.String("github-org"), c.String("github-repo"), int(issue.Number), labelsToApply)
			if err != nil {
				logrus.Error(err)
			}

		}
	})
	logrus.Fatal(http.ListenAndServe(":3000", nil))

	return nil
}
