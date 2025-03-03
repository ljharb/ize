package commands

import (
	"context"
	"errors"
	"fmt"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/hazelops/ize/internal/config"
	"github.com/hazelops/ize/internal/manager"
	"github.com/hazelops/ize/internal/manager/alias"
	"github.com/hazelops/ize/internal/manager/ecs"
	"github.com/hazelops/ize/internal/manager/serverless"
	"github.com/hazelops/ize/internal/requirements"
	"github.com/hazelops/ize/pkg/templates"
	"github.com/hazelops/ize/pkg/terminal"
	"github.com/spf13/cobra"
)

type UpAppsOptions struct {
	Config  *config.Project
	UI      terminal.UI
	Explain bool
}

var upAppsLongDesc = templates.LongDesc(`
	Build, push and deploy all apps.
`)

var upAppsExample = templates.Examples(`
	# Up all apps
	ize up apps

	# Up apps with explicitly specified config file
	ize --config-file /path/to/config up apps

	# Deploy apps with explicitly specified config file passed via environment variable
	export IZE_CONFIG_FILE=/path/to/config
	ize up apps
`)

func NewUpAppsFlags(project *config.Project) *UpAppsOptions {
	return &UpAppsOptions{
		Config: project,
	}
}

func NewCmdUpApps(project *config.Project) *cobra.Command {
	o := NewUpAppsFlags(project)

	cmd := &cobra.Command{
		Use:     "apps",
		Short:   "Manage apps deployments",
		Long:    upAppsLongDesc,
		Example: upAppsExample,
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.SilenceUsage = true
			err := o.Complete()
			if err != nil {
				return err
			}

			err = o.Validate()
			if err != nil {
				return err
			}

			err = o.Run()
			if err != nil {
				return err
			}

			return nil
		},
	}

	cmd.Flags().BoolVar(&o.Explain, "explain", false, "bash alternative shown")

	return cmd
}

func (o *UpAppsOptions) Complete() error {
	if err := requirements.CheckRequirements(requirements.WithIzeStructure(), requirements.WithConfigFile()); err != nil {
		return err
	}

	if len(o.Config.Serverless) != 0 {
		if err := requirements.CheckRequirements(requirements.WithNVM()); err != nil {
			return err
		}
	}

	o.UI = terminal.ConsoleUI(context.Background(), o.Config.PlainText)

	return nil
}

func (o *UpAppsOptions) Validate() error {
	return nil
}

func (o *UpAppsOptions) Run() error {
	ui := o.UI
	ui.Output("Deploying apps...", terminal.WithHeaderStyle())

	err := manager.InDependencyOrder(aws.BackgroundContext(), o.Config.GetApps(), func(c context.Context, name string) error {
		var err error
		if len(o.Config.AwsProfile) == 0 {
			if v, exists := o.Config.Terraform["infra"]; exists {
				o.Config.AwsProfile = v.AwsProfile
			} else {
				err = errors.New("can't detect aws_profile. Please set it via env var (AWS_PROFILE) or in ize.toml")
			}
		}

		err = deployApp(name, ui, o.Config, false)
		if err != nil {
			return err
		}

		return nil
	})
	if err != nil {
		return err
	}

	ui.Output("Deploy complete!\n", terminal.WithSuccessStyle())

	return nil
}

func deployApp(name string, ui terminal.UI, cfg *config.Project, isExplain bool) error {
	var m manager.Manager
	var icon string

	m = &ecs.Manager{
		Project: cfg,
		App:     &config.Ecs{Name: name},
	}

	if app, ok := cfg.Serverless[name]; ok {
		app.Name = name
		m = &serverless.Manager{
			Project: cfg,
			App:     app,
		}
	}
	if app, ok := cfg.Alias[name]; ok {
		app.Name = name
		m = &alias.Manager{
			Project: cfg,
			App:     app,
		}
	}
	if app, ok := cfg.Ecs[name]; ok {
		app.Name = name
		m = &ecs.Manager{
			Project: cfg,
			App:     app,
		}
	}

	if isExplain {
		return m.Explain()
	}

	if len(icon) != 0 {
		icon += " "
	}

	ui.Output("%s%s: bringing up...", icon, name, terminal.WithHeaderStyle())

	// build app container
	err := m.Build(ui)
	if err != nil {
		return fmt.Errorf("can't build app: %w", err)
	}

	// push app image
	err = m.Push(ui)
	if err != nil {
		return fmt.Errorf("can't push app: %w", err)
	}

	// deploy app image
	err = m.Deploy(ui)
	if err != nil {
		return fmt.Errorf("can't deploy app: %w", err)
	}

	ui.Output("%s%s: done.\n", icon, name, terminal.WithSuccessStyle())

	return nil
}
