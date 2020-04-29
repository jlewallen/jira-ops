package main

import (
	"flag"
	"fmt"
	"log"
	"strconv"
	"strings"

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

func shouldShow(i *jira.Issue) bool {
	if strings.ToLower(i.Fields.Status.Name) == strings.ToLower("Ready for Dev") {
		return true
	}
	if strings.ToLower(i.Fields.Status.Name) == strings.ToLower("In Progress") {
		return true
	}
	return false
}

func findEpic(jc *jira.Client, options *Options) (string, error) {
	number, err := strconv.Atoi(options.Epic)
	if err == nil {
		return fmt.Sprintf("%s-%d", options.Project, number), nil
	}

	epics, _, err := jc.Issue.Search(fmt.Sprintf("type = 'Epic' AND resolution IS EMPTY AND summary ~ \"%s*\" ORDER BY dueDate DESC", options.Epic), nil)
	if err != nil {
		log.Fatalf("error getting issues: %+v", err)
	}

	for _, e := range epics {
		log.Printf("found %s '%s'", e.Key, e.Fields.Summary)
		return e.Key, nil
	}

	return "", fmt.Errorf("unable to find Epic matching '%s'", options.Epic)
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
		desiredKey, err := findEpic(jc, options)
		if err != nil {
			log.Fatalf("error: %v", err)
		}

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
	} else {
		epics, _, err := jc.Issue.Search("type = 'Epic' AND resolution IS EMPTY ORDER BY dueDate DESC", nil)
		if err != nil {
			log.Fatalf("error getting issues: %+v", err)
		}

		for _, i := range epics {
			fmt.Printf("%+v %v (%d linked)\n", i.Key, i.Fields.Summary, len(i.Fields.IssueLinks))

			for _, link := range i.Fields.IssueLinks {
				if link.InwardIssue != nil {
					if link.InwardIssue.Fields.Resolution == nil {
						i := link.InwardIssue
						if shouldShow(i) {
							if i.Fields.Assignee != nil {
								fmt.Printf("  %s %s (%s) (%s)\n", i.Key, i.Fields.Summary, i.Fields.Status.Name, i.Fields.Assignee.Name)
							} else {
								fmt.Printf("  %s %s (%s)\n", i.Key, i.Fields.Summary, i.Fields.Status.Name)
							}
						}
					}
				}
				if link.OutwardIssue != nil {
					if link.OutwardIssue.Fields.Resolution == nil {
						i := link.OutwardIssue
						if shouldShow(i) {
							if i.Fields.Assignee != nil {
								fmt.Printf("  %s %s (%s) (%s)\n", i.Key, i.Fields.Summary, i.Fields.Status.Name, i.Fields.Assignee.Name)
							} else {
								fmt.Printf("  %s %s (%s)\n", i.Key, i.Fields.Summary, i.Fields.Status.Name)
							}
						}
					}
				}
			}

			fmt.Println()
		}
	}
}
