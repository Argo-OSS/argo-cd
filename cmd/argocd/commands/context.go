package commands

import (
	"fmt"
	"os"
	"path"
	"strings"
	"text/tabwriter"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	argocdclient "github.com/argoproj/argo-cd/v2/pkg/apiclient"
	"github.com/argoproj/argo-cd/v2/util/errors"
	"github.com/argoproj/argo-cd/v2/util/localconfig"
)

// NewContextCommand returns a new instance of an `argocd ctx` command
func NewContextCommand(clientOpts *argocdclient.ClientOptions) *cobra.Command {
	var deleteFlag bool

	// Main command definition
	command := &cobra.Command{
		Use:     "context [CONTEXT]",
		Aliases: []string{"ctx"},
		Short:   "Manage Argo CD contexts",
		Example: `# List Argo CD Contexts
argocd context list

# Use Argo CD context
argocd context use cd.argoproj.io

# Delete Argo CD context
argocd context delete cd.argoproj.io

# Switch Argo CD context (legacy)
argocd context cd.argoproj.io

# Delete Argo CD context (legacy)
argocd context cd.argoproj.io --delete`,
		Run: func(c *cobra.Command, args []string) {
			// Handle the legacy commands
			if deleteFlag {
				if len(args) == 0 {
					c.HelpFunc()(c, args)
					os.Exit(1)
				}
				err := deleteContext(args[0], clientOpts.ConfigPath)
				errors.CheckError(err)
				fmt.Printf("Deleted context '%s'\n", args[0])
				return
			}

			// Handle listing contexts when no arguments are provided
			if len(args) == 0 {
				printArgoCDContexts(clientOpts.ConfigPath)
				return
			}

			// If an argument is provided, switch to the specified context (legacy mode)
			err := useArgoCDContext(args[0], clientOpts.ConfigPath)
			errors.CheckError(err)
		},
	}

	// List subcommand
	listCommand := &cobra.Command{
		Use:   "list",
		Short: "List Argo CD contexts",
		Run: func(c *cobra.Command, args []string) {
			printArgoCDContexts(clientOpts.ConfigPath)
		},
	}

	// Use subcommand to switch context
	useCommand := &cobra.Command{
		Use:   "use [CONTEXT]",
		Short: "Switch to a specific Argo CD context",
		Args:  cobra.ExactArgs(1), // context argument is required
		Run: func(c *cobra.Command, args []string) {
			err := useArgoCDContext(args[0], clientOpts.ConfigPath)
			errors.CheckError(err)
		},
	}

	// Delete subcommand to remove context
	deleteCommand := &cobra.Command{
		Use:   "delete [CONTEXT]",
		Short: "Delete a specific Argo CD context",
		Args:  cobra.ExactArgs(1), // context argument is required
		Run: func(c *cobra.Command, args []string) {
			ctxName := args[0]

			err := deleteContext(ctxName, clientOpts.ConfigPath)
			errors.CheckError(err)
			fmt.Printf("Deleted context '%s'\n", ctxName)
		},
	}

	// Add subcommands to the main command
	command.AddCommand(listCommand)
	command.AddCommand(useCommand)
	command.AddCommand(deleteCommand)

	// Add the delete flag for backward compatibility
	command.Flags().BoolVar(&deleteFlag, "delete", false, "Delete the context instead of switching to it")

	return command
}

// Refactored logic for switching Argo CD context
func useArgoCDContext(ctxName string, configPath string) error {
	localCfg, err := localconfig.ReadLocalConfig(configPath)
	if err != nil {
		return err
	}

	argoCDDir, err := localconfig.DefaultConfigDir()
	if err != nil {
		return err
	}
	prevCtxFile := path.Join(argoCDDir, ".prev-ctx")

	// Handle "-" for previous context
	if ctxName == "-" {
		prevCtxBytes, err := os.ReadFile(prevCtxFile)
		if err != nil {
			return err
		}
		ctxName = string(prevCtxBytes)
	}

	if localCfg.CurrentContext == ctxName {
		fmt.Printf("Already at context '%s'\n", localCfg.CurrentContext)
		return nil
	}

	if _, err := localCfg.ResolveContext(ctxName); err != nil {
		return err
	}

	prevCtx := localCfg.CurrentContext
	localCfg.CurrentContext = ctxName

	// Write the updated config and previous context
	if err := localconfig.WriteLocalConfig(*localCfg, configPath); err != nil {
		return err
	}
	if err := os.WriteFile(prevCtxFile, []byte(prevCtx), 0o644); err != nil {
		return err
	}

	fmt.Printf("Switched to context '%s'\n", localCfg.CurrentContext)
	return nil
}

func deleteContext(context, configPath string) error {
	localCfg, err := localconfig.ReadLocalConfig(configPath)
	errors.CheckError(err)
	if localCfg == nil {
		return fmt.Errorf("nothing to logout from")
	}

	serverName, ok := localCfg.RemoveContext(context)
	if !ok {
		return fmt.Errorf("Context %s does not exist", context)
	}
	_ = localCfg.RemoveUser(context)
	_ = localCfg.RemoveServer(serverName)

	if localCfg.IsEmpty() {
		err = localconfig.DeleteLocalConfig(configPath)
		errors.CheckError(err)
	} else {
		if localCfg.CurrentContext == context {
			localCfg.CurrentContext = ""
		}
		err = localconfig.ValidateLocalConfig(*localCfg)
		if err != nil {
			return fmt.Errorf("Error in logging out")
		}
		err = localconfig.WriteLocalConfig(*localCfg, configPath)
		errors.CheckError(err)
	}
	fmt.Printf("Context '%s' deleted\n", context)
	return nil
}

func printArgoCDContexts(configPath string) {
	localCfg, err := localconfig.ReadLocalConfig(configPath)
	errors.CheckError(err)
	if localCfg == nil {
		log.Fatalf("No contexts defined in %s", configPath)
	}
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	defer func() { _ = w.Flush() }()
	columnNames := []string{"CURRENT", "NAME", "SERVER"}
	_, err = fmt.Fprintf(w, "%s\n", strings.Join(columnNames, "\t"))
	errors.CheckError(err)

	for _, contextRef := range localCfg.Contexts {
		context, err := localCfg.ResolveContext(contextRef.Name)
		if err != nil {
			log.Warnf("Context '%s' had error: %v", contextRef.Name, err)
		}
		prefix := " "
		if localCfg.CurrentContext == context.Name {
			prefix = "*"
		}
		_, err = fmt.Fprintf(w, "%s\t%s\t%s\n", prefix, context.Name, context.Server.Server)
		errors.CheckError(err)
	}
}
