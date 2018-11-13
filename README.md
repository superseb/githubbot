githubbot
========

## Using

Parameters
* `webhooktoken-file`: Webhook token used to validate incoming requests triggered by the webhook on the repository
* `patoken-file`: Personal Access Token used to add new labels to incoming issues
* `github-org`: GitHub organization name of the repository where the webhook is configured
* `github-repo`: GitHub repository name of the repository where the webhook is configured

## Building

`make`

## Running

`./bin/githubbot --webhooktoken-file=WEBHOOKTOKENFILE --patoken-file=PATOKENFILE --github-org ORG --github-repo REPO`
