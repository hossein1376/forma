package cli

import (
	"bufio"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"strings"

	"github.com/hossein1376/forma"
	"github.com/hossein1376/forma/templates/gocli"
	"github.com/hossein1376/forma/templates/gohttp"
	"github.com/hossein1376/forma/templates/golibrary"
)

const (
	exitOK      = 0
	exitFailure = 1
	exitUsage   = 2
)

type commandOptions struct {
	name      string
	module    string
	port      int
	docker    bool
	githubCI  bool
	yes       bool
	overwrite bool
}

func Run(
	ctx context.Context,
	args []string,
	stdin io.Reader,
	stdout, stderr io.Writer,
) int {
	if len(args) < 2 {
		printUsage(stderr)
		return exitUsage
	}

	action := args[0]
	templateName := args[1]
	if action != "plan" && action != "create" {
		fmt.Fprintf(stderr, "unknown command %q\n", action)
		printUsage(stderr)
		return exitUsage
	}
	if !knownTemplate(templateName) {
		fmt.Fprintf(stderr, "unknown template %q\n", templateName)
		return exitUsage
	}

	options, positional, ok := parseOptions(action, args[2:], stderr)
	if !ok {
		return exitUsage
	}
	if action == "plan" && len(positional) != 0 {
		fmt.Fprintln(stderr, "plan does not accept a destination")
		return exitUsage
	}
	if action == "create" && len(positional) != 1 {
		fmt.Fprintln(stderr, "create requires exactly one destination")
		return exitUsage
	}

	plan, err := buildPlan(templateName, options)
	if err != nil {
		fmt.Fprintf(stderr, "build plan: %v\n", err)
		return exitFailure
	}

	if action == "plan" {
		printPlan(stdout, plan)
		return exitOK
	}

	destination := positional[0]
	changes, err := forma.Inspect(ctx, plan, destination)
	printChanges(stdout, changes)
	if err != nil {
		fmt.Fprintf(stderr, "inspect destination: %v\n", err)
		return exitFailure
	}
	if !options.overwrite &&
		containsOverwrite(changes) {
		fmt.Fprintln(
			stderr, "existing files differ; use --overwrite to replace them",
		)
		return exitFailure
	}
	if !options.yes &&
		!confirm(stdin, stdout) {
		fmt.Fprintln(stdout, "Cancelled.")
		return exitOK
	}

	policy := forma.FailOnExisting
	if options.overwrite {
		policy = forma.OverwriteExisting
	}
	result, err := forma.Apply(
		ctx,
		plan,
		destination,
		forma.ApplyOptions{
			ExistingFilePolicy: policy,
		},
	)
	printResult(stdout, result)
	if err != nil {
		fmt.Fprintf(stderr, "apply plan: %v\n", err)
		return exitFailure
	}
	return exitOK
}

func knownTemplate(name string) bool {
	switch name {
	case "go-http-service", "go-cli", "go-library":
		return true
	default:
		return false
	}
}

func buildPlan(
	templateName string,
	options commandOptions,
) (forma.Plan, error) {
	switch templateName {
	case "go-http-service":
		return forma.PlanFor(
			gohttp.Template{},
			gohttp.Input{
				Name:       options.name,
				ModulePath: options.module,
				Port:       options.port,
				Docker:     options.docker,
				GitHubCI:   options.githubCI,
			},
		)
	case "go-cli":
		if options.docker {
			return forma.Plan{}, fmt.Errorf(
				"--docker is only valid for go-http-service",
			)
		}
		return forma.PlanFor(
			gocli.Template{},
			gocli.Input{
				Name:       options.name,
				ModulePath: options.module,
				GitHubCI:   options.githubCI,
			},
		)
	case "go-library":
		if options.docker {
			return forma.Plan{}, fmt.Errorf(
				"--docker is only valid for go-http-service",
			)
		}
		return forma.PlanFor(
			golibrary.Template{},
			golibrary.Input{
				Name:       options.name,
				ModulePath: options.module,
				GitHubCI:   options.githubCI,
			},
		)
	default:
		return forma.Plan{}, fmt.Errorf(
			"unknown template %q",
			templateName,
		)
	}
}

func containsOverwrite(changes forma.Changes) bool {
	for _, change := range changes {
		if change.Kind == forma.ChangeOverwrite {
			return true
		}
	}
	return false
}

func parseOptions(
	action string, args []string, stderr io.Writer,
) (commandOptions, []string, bool) {
	var options commandOptions
	flags := flag.NewFlagSet(action, flag.ContinueOnError)
	flags.SetOutput(stderr)
	flags.StringVar(&options.name, "name", "", "service name")
	flags.StringVar(&options.module, "module", "", "Go module path")
	flags.IntVar(&options.port, "port", 8080, "HTTP listen port")
	flags.BoolVar(&options.docker, "docker", false, "include a Dockerfile")
	flags.BoolVar(
		&options.githubCI,
		"github-ci",
		false,
		"include a GitHub Actions workflow",
	)
	if action == "create" {
		flags.BoolVar(&options.yes, "yes", false, "skip confirmation")
		flags.BoolVar(
			&options.overwrite,
			"overwrite",
			false,
			"replace differing regular files",
		)
	}
	flags.Usage = func() {
		printCommandUsage(stderr, action)
	}
	if err := flags.Parse(args); err != nil {
		return commandOptions{}, nil, false
	}
	return options, flags.Args(), true
}

func printPlan(writer io.Writer, plan forma.Plan) {
	fmt.Fprintln(writer, "Plan:")
	for _, artifact := range plan.Artifacts() {
		kind := "file"
		if artifact.Kind() == forma.ArtifactDirectory {
			kind = "directory"
		}
		fmt.Fprintf(
			writer,
			"  %-9s %04o %s\n",
			kind,
			artifact.Mode().Perm(),
			artifact.Path(),
		)
	}
}

func printChanges(writer io.Writer, changes forma.Changes) {
	fmt.Fprintln(writer, "Changes:")
	for _, change := range changes {
		fmt.Fprintf(writer, "  %-9s %s\n", changeKind(change.Kind), change.Path)
	}
}

func changeKind(kind forma.ChangeKind) string {
	switch kind {
	case forma.ChangeCreate:
		return "create"
	case forma.ChangeOverwrite:
		return "overwrite"
	case forma.ChangeUnchanged:
		return "unchanged"
	case forma.ChangeConflict:
		return "conflict"
	case forma.ChangeUpdatePermissions:
		return "chmod"
	default:
		return "unknown"
	}
}

func printResult(writer io.Writer, result forma.ApplyResult) {
	var changed int
	for _, outcome := range result.Outcomes {
		if outcome.Status == forma.ApplyCreated ||
			outcome.Status == forma.ApplyOverwritten ||
			outcome.Status == forma.ApplyPermissionsUpdated {
			changed++
		}
	}
	fmt.Fprintf(writer, "Applied %d artifact(s).\n", changed)
}

func confirm(reader io.Reader, writer io.Writer) bool {
	fmt.Fprint(writer, "Apply these changes? [y/N] ")
	line, err := bufio.NewReader(reader).ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return false
	}
	answer := strings.ToLower(strings.TrimSpace(line))
	return answer == "y" || answer == "yes"
}

func printUsage(writer io.Writer) {
	fmt.Fprintln(writer, "usage: forma <plan|create> <template> [options]")
	fmt.Fprintln(writer, "templates: go-http-service, go-cli, go-library")
	fmt.Fprintln(writer)
	printCommandUsage(writer, "plan")
	printCommandUsage(writer, "create")
}

func printCommandUsage(writer io.Writer, action string) {
	destination := ""
	if action == "create" {
		destination = " <destination>"
	}
	fmt.Fprintf(
		writer,
		"  forma %s <template> --name <name> --module <path>%s\n",
		action,
		destination,
	)
}
