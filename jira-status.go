package main

import (
	"flag"
	"fmt"
	"log"
	"regexp"
	"strings"

	"github.com/andygrunwald/go-jira"
)

type Options struct {
	Project string
	Version string
	Upkeep  bool
	Pending bool
	Help    bool
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

func displaySearch(jc *jira.Client, search string) error {
	issues, _, err := jc.Issue.Search(search, nil)
	if err != nil {
		return fmt.Errorf("error getting issues: %+v", err)
	}

	for _, i := range issues {
		if i.Fields.Assignee != nil {
			fmt.Printf("%-8s %s (%s) (%s)\n", i.Key, i.Fields.Summary, i.Fields.Status.Name, i.Fields.Assignee.Name)
		} else {
			fmt.Printf("%-8s %s (%s)\n", i.Key, i.Fields.Summary, i.Fields.Status.Name)
		}
	}

	return nil
}

func displayIssues(jc *jira.Client, options *Options) error {
	epics, _, err := jc.Issue.Search("type = 'Epic' AND resolution IS EMPTY ORDER BY dueDate DESC", nil)
	if err != nil {
		return fmt.Errorf("error getting issues: %+v", err)
	}

	for _, i := range epics {
		fmt.Printf("%-8s %v (%d linked)\n", i.Key, i.Fields.Summary, len(i.Fields.IssueLinks))

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

	return nil
}

var imagesRegexp = regexp.MustCompile("!.+!")

func makeAllImagesThumbnails(body string) (string, error) {
	newBody := imagesRegexp.ReplaceAllStringFunc(body, func(match string) string {
		pipes := strings.Split(match, "|")
		if len(pipes) == 2 {
			return match
		}

		nameOnly := strings.Replace(match, "!", "", -1)

		return fmt.Sprintf("!%s|thumbnail!", nameOnly)
	})

	return newBody, nil
}

func findVersion(jc *jira.Client, projectKey, search string) (version *jira.Version, err error) {
	project, _, err := jc.Project.Get(projectKey)
	if err != nil {
		return nil, err
	}

	matches := make([]jira.Version, 0)

	for _, v := range project.Versions {
		if !v.Archived && !v.Released {
			if strings.Contains(v.Name, search) {
				matches = append(matches, v)
			}
		}
	}

	if len(matches) == 0 {
		return nil, fmt.Errorf("no such version in project: %s / %s", projectKey, search)
	}

	return &matches[0], nil
}

func reversion(jc *jira.Client, options *Options) error {
	version, err := findVersion(jc, options.Project, options.Version)
	if err != nil {
		return err
	}

	if false {
		log.Printf("version: %v", version.Name)
	}

	for _, issueNumber := range flag.Args() {
		issueKey := fmt.Sprintf("%s-%s", options.Project, issueNumber)

		issue, _, err := jc.Issue.Get(issueKey, nil)
		if err != nil {
			return fmt.Errorf("error getting issue: %+v", err)
		}

		necessary := true

		for _, fv := range issue.Fields.FixVersions {
			if fv.ID == version.ID {
				necessary = false
				break
			}
		}

		if necessary {
			log.Printf("moving %s to version %s", issueKey, version.Name)

			update := &jira.Issue{
				Key: issue.Key,
				Fields: &jira.IssueFields{
					FixVersions: []*jira.FixVersion{
						&jira.FixVersion{
							ID: version.ID,
						},
					},
				},
			}

			if _, _, err := jc.Issue.Update(update); err != nil {
				return fmt.Errorf("error updating description: %+v", err)
			}
		}
	}

	return nil
}

func upkeep(jc *jira.Client, options *Options) error {
	issues, _, err := jc.Issue.Search("resolution IS EMPTY ORDER BY updated DESC", nil)
	if err != nil {
		return fmt.Errorf("error getting issues: %+v", err)
	}

	for _, i := range issues {
		if false {
			fmt.Printf("%+v", i.Fields.Description)
		}

		issue, _, err := jc.Issue.Get(i.Key, nil)
		if err != nil {
			return fmt.Errorf("error getting issue: %+v", err)
		}

		newDescription, err := makeAllImagesThumbnails(i.Fields.Description)
		if err != nil {
			return fmt.Errorf("error changing thumbnails: %+v", err)
		}

		if newDescription != i.Fields.Description {
			fmt.Printf("%-8s %v (%d linked)\n", i.Key, i.Fields.Summary, len(i.Fields.IssueLinks))

			update := &jira.Issue{
				Key: i.Key,
				Fields: &jira.IssueFields{
					Description: newDescription,
				},
			}

			if _, _, err := jc.Issue.Update(update); err != nil {
				return fmt.Errorf("error updating description: %+v", err)
			}
		}

		for _, c := range issue.Fields.Comments.Comments {
			newBody, err := makeAllImagesThumbnails(c.Body)
			if err != nil {
				return fmt.Errorf("error changing thumbnails: %+v", err)
			}

			if newBody != c.Body {
				fmt.Printf("%+v %v (%d linked)\n", i.Key, i.Fields.Summary, len(i.Fields.IssueLinks))

				c.Body = newBody

				if _, _, err := jc.Issue.UpdateComment(i.Key, c); err != nil {
					return fmt.Errorf("error updating: %+v", err)
				}
			}
		}
	}

	return nil
}

func main() {
	options := &Options{}
	flag.StringVar(&options.Project, "project", "FK", "default project prefix, should rarely change")
	flag.StringVar(&options.Version, "version", "", "version to link issues to")
	flag.BoolVar(&options.Upkeep, "upkeep", false, "fix thumbnails on recently modified issues")
	flag.BoolVar(&options.Pending, "pending", false, "issues ready for deploy")
	flag.BoolVar(&options.Help, "help", false, "help")
	flag.Parse()

	if options.Help {
		flag.Usage()
		return
	}

	jc, err := jira.NewClient(nil, JiraUrl)
	if err != nil {
		fmt.Printf("error creating client: %+v\n", err)
		return
	}

	res, err := jc.Authentication.AcquireSessionCookie(JiraUsername, JiraPassword)
	if err != nil || res == false {
		log.Fatalf("error authenticating: %+v", err)
	}

	if options.Upkeep {
		log.Printf("querying for issues")

		if err := upkeep(jc, options); err != nil {
			log.Fatalf("error: %v", err)
		}
		return
	}

	if options.Pending {
		search := `status IN ("Ready to Deploy")`
		if err := displaySearch(jc, search); err != nil {
			log.Fatalf("error: %v", err)
		}
		return
	}

	if len(options.Version) > 0 {
		if err := reversion(jc, options); err != nil {
			log.Fatalf("error: %v", err)
		}

		return
	}

	search := `status NOT IN ("Awaiting QA") AND
				       type != Epic AND resolution is EMPTY AND
				       (assignee = currentUser() OR assignee WAS currentUser() OR reporter = currentUser() OR comment ~ currentUser() OR watcher = currentUser()) ORDER BY updated DESC`
	if err := displaySearch(jc, search); err != nil {
		log.Fatalf("error: %v", err)
	}
}
