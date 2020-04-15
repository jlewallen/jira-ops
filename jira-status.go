package main

import (
	"flag"
	"fmt"
	"log"

	"github.com/andygrunwald/go-jira"
)

type Options struct {
	Project string
	Epic    string
}

func deleteLink(jc *jira.Client, linkId string) error {
	req, _ := jc.NewRequest("DELETE", "/rest/api/2/issueLink/"+linkId, nil)
	_, err := jc.Do(req, nil)
	if err != nil {
		return err
	}

	return nil
}

func main() {
	options := &Options{}
	flag.StringVar(&options.Project, "project", "FK", "project prefix")
	flag.StringVar(&options.Epic, "epic", "", "target epic")
	flag.Parse()

	jc, err := jira.NewClient(nil, JiraUrl)
	if err != nil {
		fmt.Printf("error creating client: %+v\n", err)
		return
	}

	res, err := jc.Authentication.AcquireSessionCookie(JiraUsername, JiraPassword)
	if err != nil || res == false {
		log.Fatalf("error authenticating: %+v", err)
	}

	if len(options.Epic) > 0 {
		desiredKey := fmt.Sprintf("%s-%s", options.Project, options.Epic)

		types, _, _ := jc.IssueLinkType.GetList()
		for _, a := range types {
			log.Printf("OK: ", a)
		}

		for _, issueNumber := range flag.Args() {
			issueKey := fmt.Sprintf("%s-%s", options.Project, issueNumber)

			log.Printf("moving %s to epic %s", issueKey, desiredKey)

			issue, _, err := jc.Issue.Get(issueKey, nil)
			if err != nil {
				log.Fatalf("error getting issue: %+v", err)
			}

			linked := false

			for _, link := range issue.Fields.IssueLinks {
				if link.InwardIssue != nil {
					if link.InwardIssue.Key == desiredKey {
						linked = true
					} else {
						if link.InwardIssue.Fields.Type.Name == "Epic" {
							log.Printf("removing (inward) link to: %s", link.InwardIssue.Key)
							err := deleteLink(jc, link.ID)
							if err != nil {
								log.Fatalf("error: %v", err)
							}
						}
					}
				}
				if link.OutwardIssue != nil {
					if link.OutwardIssue.Key == desiredKey {
						linked = true
					} else {
						if link.OutwardIssue.Fields.Type.Name == "Epic" {
							log.Printf("removing (outward) link to: %s", link.OutwardIssue.Key)
							err := deleteLink(jc, link.ID)
							if err != nil {
								log.Fatalf("error: %v", err)
							}
						}
					}
				}
			}

			if !linked {
				log.Printf("adding link from %s to %s", issueKey, desiredKey)

				newLink := &jira.IssueLink{
					Type: jira.IssueLinkType{
						Name: "Relates",
					},
					InwardIssue: &jira.Issue{
						Key: desiredKey,
					},
					OutwardIssue: &jira.Issue{
						Key: issueKey,
					},
				}
				_, err := jc.Issue.AddLink(newLink)
				if err != nil {
					log.Fatalf("error: %v", err)
				}
			}
		}
	}

	if false {
		critical, _, err := jc.Issue.Search("priority = 'Highest'", nil)
		if err != nil {
			log.Fatalf("error getting issues: %+v", err)
		}

		fmt.Printf("%+v\n", critical)

		for _, i := range critical {
			log.Printf("%+v", i.Fields)
		}

		blocked, _, err := jc.Issue.Search("status = 'Blocked'", nil)
		if err != nil {
			log.Fatalf("error getting issues: %+v", err)
		}

		fmt.Printf("%+v\n", blocked)
	}
}
