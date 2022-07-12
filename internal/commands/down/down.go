package down

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/hazelops/ize/internal/apps"
	"github.com/hazelops/ize/internal/apps/ecs"
	"github.com/hazelops/ize/internal/config"
	"github.com/hazelops/ize/internal/terraform"
	"github.com/hazelops/ize/pkg/templates"
	"github.com/hazelops/ize/pkg/terminal"
	"github.com/pterm/pterm"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

type DownOptions struct {
	Config           *config.Project
	AppName          string
	SkipBuildAndPush bool
	AutoApprove      bool
	ui               terminal.UI
}

type Apps map[string]*interface{}

var downLongDesc = templates.LongDesc(`
	Destroy infrastructure or application.
	For app destroy the app name must be specified.
`)

var downExample = templates.Examples(`
	# Destroy all (config file required)
	ize down

	# Destroy app (config file required)
	ize down <app name>

	# Destroy app with explicitly specified config file
	ize --config-file (or -c) /path/to/config down <app name>

	# Destroy app with explicitly specified config file passed via environment variable.
	export IZE_CONFIG_FILE=/path/to/config
	ize down <app name>
`)

func NewDownFlags() *DownOptions {
	return &DownOptions{}
}

func NewCmdDown() *cobra.Command {
	o := NewDownFlags()

	cmd := &cobra.Command{
		Use:     "down [flags] [app name]",
		Example: downExample,
		Short:   "Destroy application",
		Long:    downLongDesc,
		Args:    cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.SilenceUsage = true
			if len(args) == 0 && !o.AutoApprove {
				pterm.Warning.Println("Please set the flag: --auto-approve")
				return nil
			}

			err := o.Complete(cmd, args)
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

	cmd.Flags().BoolVar(&o.AutoApprove, "auto-approve", false, "approve deploy all")

	cmd.AddCommand(
		NewCmdDownInfra(),
	)

	return cmd
}

func (o *DownOptions) Complete(cmd *cobra.Command, args []string) error {
	var err error

	if len(args) == 0 {
		if err := config.CheckRequirements(config.WithConfigFile()); err != nil {
			return err
		}
		o.Config, err = config.GetConfig()
		if err != nil {
			return fmt.Errorf("can't load options for a command: %w", err)
		}

		if o.Config.Terraform == nil {
			o.Config.Terraform = map[string]*config.Terraform{}
			o.Config.Terraform["infra"] = &config.Terraform{}
		}

		if len(o.Config.Terraform["infra"].AwsProfile) == 0 {
			o.Config.Terraform["infra"].AwsProfile = o.Config.AwsProfile
		}

		if len(o.Config.Terraform["infra"].AwsRegion) == 0 {
			o.Config.Terraform["infra"].AwsProfile = o.Config.AwsRegion
		}

		if len(o.Config.Terraform["infra"].Version) == 0 {
			o.Config.Terraform["infra"].Version = o.Config.TerraformVersion
		}
	} else {
		o.Config, err = config.GetConfig()
		if err != nil {
			return fmt.Errorf("can't load options for a command: %w", err)
		}

		o.AppName = cmd.Flags().Args()[0]
	}

	o.ui = terminal.ConsoleUI(context.Background(), o.Config.PlainText)

	return nil
}

func (o *DownOptions) Validate() error {
	if o.AppName == "" {
		err := validateAll(o)
		if err != nil {
			return err
		}
	} else {
		err := validate(o)
		if err != nil {
			return err
		}
	}

	return nil
}

func (o *DownOptions) Run() error {
	ui := o.ui
	if o.AppName == "" {
		err := destroyAll(ui, o)
		if err != nil {
			return err
		}
	} else {
		err := destroyApp(ui, o)
		if err != nil {
			return err
		}
	}

	return nil
}

func validate(o *DownOptions) error {
	if len(o.Config.Env) == 0 {
		return fmt.Errorf("can't validate options: env must be specified")
	}

	if len(o.Config.Namespace) == 0 {
		return fmt.Errorf("can't validate options: namespace must be specified")
	}

	if len(o.AppName) == 0 {
		return fmt.Errorf("can't validate options: app name be specified")
	}

	return nil
}

func validateAll(o *DownOptions) error {
	if len(o.Config.Env) == 0 {
		return fmt.Errorf("can't validate options: env must be specified")
	}

	if len(o.Config.Namespace) == 0 {
		return fmt.Errorf("can't validate options: namespace must be specified")
	}

	return nil
}

func destroyAll(ui terminal.UI, o *DownOptions) error {

	ui.Output("Destroying apps...", terminal.WithHeaderStyle())
	sg := ui.StepGroup()
	defer sg.Wait()

	err := apps.InReversDependencyOrder(aws.BackgroundContext(), o.Config.GetApps(), func(c context.Context, name string) error {
		o.Config.AwsProfile = o.Config.Terraform["infra"].AwsProfile

		var appService apps.App

		if app, ok := o.Config.Serverless[name]; ok {
			appService = &apps.SlsService{
				Project: o.Config,
				App:     app,
			}
		}
		if app, ok := o.Config.Alias[name]; ok {
			appService = &apps.AliasService{
				Project: o.Config,
				App:     app,
			}
		}
		if app, ok := o.Config.Ecs[name]; ok {
			appService = &ecs.EcsService{
				Project: o.Config,
				App:     app,
			}
		}

		// destroy
		err := appService.Destroy(ui)
		if err != nil {
			return fmt.Errorf("can't destroy app: %w", err)
		}

		return nil
	})
	if err != nil {
		return err
	}

	var tf terraform.Terraform

	logrus.Infof("infra: %s", o.Config.Terraform["infra"])

	v, err := o.Config.Session.Config.Credentials.Get()
	if err != nil {
		return fmt.Errorf("can't set AWS credentials: %w", err)
	}

	env := []string{
		fmt.Sprintf("ENV=%v", o.Config.Env),
		fmt.Sprintf("AWS_PROFILE=%v", o.Config.Terraform["infra"].AwsProfile),
		fmt.Sprintf("TF_LOG=%v", o.Config.TFLog),
		fmt.Sprintf("TF_LOG_PATH=%v", o.Config.TFLogPath),
		fmt.Sprintf("AWS_ACCESS_KEY_ID=%v", v.AccessKeyID),
		fmt.Sprintf("AWS_SECRET_ACCESS_KEY=%v", v.SecretAccessKey),
		fmt.Sprintf("AWS_SESSION_TOKEN=%v", v.SessionToken),
	}

	switch o.Config.PreferRuntime {
	case "docker":
		tf = terraform.NewDockerTerraform(o.Config.Terraform["infra"].Version, []string{"destroy", "-auto-approve"}, env, nil, o.Config.Home, o.Config.InfraDir, o.Config.EnvDir)
	case "native":
		tf = terraform.NewLocalTerraform(o.Config.Terraform["infra"].Version, []string{"destroy", "-auto-approve"}, env, nil, o.Config.EnvDir)
		err = tf.Prepare()
		if err != nil {
			return fmt.Errorf("can't destroy all: %w", err)
		}
	default:
		return fmt.Errorf("can't supported %s runtime", o.Config.PreferRuntime)
	}

	ui.Output("Running destroy infra...", terminal.WithHeaderStyle())
	ui.Output("Execution terraform destroy...", terminal.WithHeaderStyle())

	err = tf.RunUI(ui)
	if err != nil {
		return fmt.Errorf("can't destroy all: %w", err)
	}

	ui.Output("Destroy all completed!\n", terminal.WithSuccessStyle())

	return nil
}

func destroyApp(ui terminal.UI, o *DownOptions) error {
	ui.Output("Destroying %s app...\n", o.AppName, terminal.WithHeaderStyle())
	sg := ui.StepGroup()
	defer sg.Wait()

	var appService apps.App

	if app, ok := o.Config.Serverless[o.AppName]; ok {
		appService = &apps.SlsService{
			Project: o.Config,
			App:     app,
		}
	}
	if app, ok := o.Config.Alias[o.AppName]; ok {
		appService = &apps.AliasService{
			Project: o.Config,
			App:     app,
		}
	}
	if app, ok := o.Config.Ecs[o.AppName]; ok {
		appService = &ecs.EcsService{
			Project: o.Config,
			App:     app,
		}
	}

	err := appService.Destroy(ui)
	if err != nil {
		return fmt.Errorf("can't down: %w", err)
	}

	ui.Output("Destroy app %s completed\n", o.AppName, terminal.WithSuccessStyle())

	return nil
}
