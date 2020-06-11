package main

import (
	"fmt"
	"log"
	"strconv"

	"github.com/andygrunwald/go-jira"
)

func findEpic(jc *jira.Client, project, epic string) (string, error) {
	number, err := strconv.Atoi(epic)
	if err == nil {
		return fmt.Sprintf("%s-%d", project, number), nil
	}

	epics, _, err := jc.Issue.Search(fmt.Sprintf("type = 'Epic' AND resolution IS EMPTY AND summary ~ \"%s*\" ORDER BY dueDate DESC", epic), nil)
	if err != nil {
		log.Fatalf("error getting issues: %+v", err)
	}

	for _, e := range epics {
		log.Printf("found %s '%s'", e.Key, e.Fields.Summary)
		return e.Key, nil
	}

	return "", fmt.Errorf("unable to find Epic matching '%s'", epic)
}

func linkEpics(jc *jira.Client, project, epic string) error {
	desiredKey, err := findEpic(jc, project, epic)
	if err != nil {
		return err
	}

	types, _, _ := jc.IssueLinkType.GetList()
	for _, a := range types {
		log.Printf("OK: ", a)
	}

	for _, issueNumber := range []string{ /* issues */ } {
		issueKey := fmt.Sprintf("%s-%s", project, issueNumber)

		log.Printf("moving %s to epic %s", issueKey, desiredKey)

		issue, _, err := jc.Issue.Get(issueKey, nil)
		if err != nil {
			return fmt.Errorf("error getting issue: %+v", err)
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
							return fmt.Errorf("error: %v", err)
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
							return fmt.Errorf("error: %v", err)
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
				return fmt.Errorf("error: %v", err)
			}
		}
	}

	return nil
}
