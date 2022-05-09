package tunnel

import (
	"bytes"
	"fmt"
	"io/fs"
	"os/exec"
	"syscall"

	"github.com/hazelops/ize/internal/config"
	"github.com/pterm/pterm"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

type TunnelDownOptions struct {
	Config *config.Config
}

func NewTunnelDownOptions() *TunnelDownOptions {
	return &TunnelDownOptions{}
}

func NewCmdTunnelDown() *cobra.Command {
	o := NewTunnelDownOptions()

	cmd := &cobra.Command{
		Use:   "down",
		Short: "Close tunnel",
		Long:  "Close tunnel",
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.SilenceUsage = true

			err := o.Complete(cmd, args)
			if err != nil {
				return err
			}

			err = o.Validate()
			if err != nil {
				return err
			}

			err = o.Run(cmd)
			if err != nil {
				return err
			}

			return nil
		},
	}

	return cmd
}

func (o *TunnelDownOptions) Complete(cmd *cobra.Command, args []string) error {
	cfg, err := config.GetConfig()
	if err != nil {
		return fmt.Errorf("can't complete options: %w", err)
	}

	o.Config = cfg

	return nil
}

func (o *TunnelDownOptions) Validate() error {
	if len(o.Config.Env) == 0 {
		return fmt.Errorf("env must be specified")
	}

	return nil
}

func (o *TunnelDownOptions) Run(cmd *cobra.Command) error {
	c := exec.Command(
		"ssh", "-S", "bastion.sock", "-O", "exit", "",
	)
	out := &bytes.Buffer{}
	c.Stdout = out
	c.Stderr = out
	c.Dir = viper.GetString("ENV_DIR")

	fmt.Println(viper.GetString("ENV_DIR"))

	err := c.Run()
	if err != nil {
		patherr, ok := err.(*fs.PathError)
		if ok {
			return fmt.Errorf("unable to access folder '%s': %w", c.Dir, patherr.Err)
		}
		exiterr := err.(*exec.ExitError)
		status := exiterr.Sys().(syscall.WaitStatus)
		if status.ExitStatus() != 255 {
			logrus.Debug(out.String())
			return fmt.Errorf("unable to bring the tunnel down: %w", err)
		}
		return fmt.Errorf("unable to bring the tunnel down: tunnel is not active")
	}

	pterm.Success.Println("Tunnel is down!")

	return nil
}
