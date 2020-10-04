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
	Project        string
	Version        string
	Search         string
	Upkeep         bool
	Pending        bool
	Progress       bool
	DeployedPortal bool
	DeployedApp    bool
	Help           bool
	Pull           string
}

func echoIssueActionMessage(action string, issue *jira.Issue) {
	log.Printf("%v %v: '%v'\n", action, issue.Key, issue.Fields.Summary)
}

func echoIssueStatusMessage(issue *jira.Issue) {
	fmt.Printf("%-8s %-18s %s\n", issue.Key, issue.Fields.Status.Name, issue.Fields.Summary)
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

	for _, issue := range issues {
		echoIssueStatusMessage(&issue)
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

var imagesRegexp = regexp.MustCompile("![^!\n]+!")

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

	enabled := true

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

			if enabled {
				if _, _, err := jc.Issue.Update(update); err != nil {
					return fmt.Errorf("error updating description: %+v", err)
				}
			}

			fmt.Printf("OLD: '%v'\n", i.Fields.Description)
			fmt.Printf("NEW: '%v'\n", newDescription)
		}

		for _, c := range issue.Fields.Comments.Comments {
			newBody, err := makeAllImagesThumbnails(c.Body)
			if err != nil {
				return fmt.Errorf("error changing thumbnails: %+v", err)
			}

			if newBody != c.Body {
				fmt.Printf("%+v %v (%d linked)\n", i.Key, i.Fields.Summary, len(i.Fields.IssueLinks))

				if enabled {
					c.Body = newBody
					if _, _, err := jc.Issue.UpdateComment(i.Key, c); err != nil {
						return fmt.Errorf("error updating: %+v", err)
					}
				}
				fmt.Printf("OLD: '%v'\n", c.Body)
				fmt.Printf("NEW: '%v'\n", newBody)
			}
		}
	}

	return nil
}

func changeStatus(jc *jira.Client, options *Options, search, desired string) error {
	issues, _, err := jc.Issue.Search(search, nil)
	if err != nil {
		return fmt.Errorf("error getting issues: %+v", err)
	}

	for _, i := range issues {
		if false {
			fmt.Printf("%-8s %-18s %s\n", i.Key, i.Fields.Status.Name, i.Fields.Summary)
		}

		if err := changeIssueStatus(jc, &i, desired); err != nil {
			return err
		}
	}

	return nil
}

func changeIssueStatus(jc *jira.Client, issue *jira.Issue, desired string) error {
	transitions, _, err := jc.Issue.GetTransitions(issue.ID)
	if err != nil {
		return err
	}

	for _, transition := range transitions {
		if transition.To.Name == desired {
			echoIssueActionMessage("changing", issue)
			if _, err := jc.Issue.DoTransition(issue.ID, transition.ID); err != nil {
				return fmt.Errorf("error updating status: %+v", err)
			}
			return nil
		}
	}

	return fmt.Errorf("missing transition")
}

func findIssue(jc *jira.Client, search string) (*jira.Issue, error) {
	issues, _, err := jc.Issue.Search(search, nil)
	if err != nil {
		return nil, fmt.Errorf("error getting issues: %+v", err)
	}

	if len(issues) != 1 {
		return nil, fmt.Errorf("unable to find issue")
	}

	return &issues[0], nil
}

func pullIssue(jc *jira.Client, issue *jira.Issue) error {
	return changeIssueStatus(jc, issue, "In Progress")
}

func main() {
	options := &Options{}
	flag.StringVar(&options.Project, "project", "FK", "default project prefix, should rarely change")
	flag.StringVar(&options.Version, "version", "", "version to link issues to")
	flag.StringVar(&options.Pull, "pull", "", "pull a card to start working")
	flag.StringVar(&options.Search, "search", "", "search cards")
	flag.BoolVar(&options.Progress, "progress", false, "display mine in progress")
	flag.BoolVar(&options.Upkeep, "upkeep", false, "fix thumbnails on recently modified issues")
	flag.BoolVar(&options.Pending, "pending", false, "issues ready for deploy")
	flag.BoolVar(&options.DeployedPortal, "deployed-portal", false, "deployed portal")
	flag.BoolVar(&options.DeployedApp, "deployed-app", false, "deployed app")
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

	if options.Progress {
		search := fmt.Sprintf(`(project = 'FK') AND (status = 'In Progress') AND (assignee = currentUser())`)
		if err := displaySearch(jc, search); err != nil {
			log.Fatalf("error: %v", err)
		}
		return
	}

	if options.Search != "" {
		search := fmt.Sprintf(`(project = 'FK') AND (resolution IS EMPTY) AND (summary ~ '%s*')`, options.Search)
		// log.Printf("searching: %s", search)
		if err := displaySearch(jc, search); err != nil {
			log.Fatalf("error: %v", err)
		}
		return
	}

	if options.Pull != "" {
		issueKey := fmt.Sprintf("%s-%s", options.Project, options.Pull)
		search := fmt.Sprintf(`(key = '%s')`, issueKey)
		issue, err := findIssue(jc, search)
		if err != nil {
			log.Fatalf("error: %v", err)
		}
		if err := pullIssue(jc, issue); err != nil {
			log.Fatalf("error: %v", err)
		}
		return
	}

	if options.Pending {
		search := `status IN ("Ready for Deploy") AND component IN ("Portal", "Backend", "Mobile App")`
		if err := displaySearch(jc, search); err != nil {
			log.Fatalf("error: %v", err)
		}
		return
	}

	if options.DeployedPortal {
		search := `status IN ("Ready for Deploy") AND component IN ("Portal", "Backend")`
		if err := changeStatus(jc, options, search, "Awaiting QA"); err != nil {
			log.Fatalf("error: %v", err)
		}
		return
	}

	if options.DeployedApp {
		search := `status IN ("Ready for Deploy") AND component IN ("Mobile App")`
		if err := changeStatus(jc, options, search, "Awaiting QA"); err != nil {
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

	search := `(status NOT IN ("Awaiting QA")) AND
			   (type != Epic) AND
			   (resolution is EMPTY) AND
			   (project IN ('FK')) AND
			   (assignee = currentUser() OR assignee WAS currentUser() OR reporter = currentUser() OR comment ~ currentUser() OR watcher = currentUser())
		       ORDER BY updated DESC`
	if err := displaySearch(jc, search); err != nil {
		log.Fatalf("error: %v", err)
	}
}
