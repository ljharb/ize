package commands

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/pterm/pterm"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func NewCmdAWSProfile() *cobra.Command {
	cmd := &cobra.Command{
		Use:                   "aws-profile",
		Short:                 "Configure aws profile",
		DisableFlagsInUseLine: true,
		Long:                  "Configure new aws profile from environment variables",
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.SilenceUsage = true

			credentialsPath, err := ConfigureAwsProfile()
			if err != nil {
				return err
			}

			pterm.Success.Printfln("AWS profile `%s` added to %s", viper.GetString("AWS_PROFILE"), credentialsPath)

			return nil
		},
	}

	return cmd
}

func ConfigureAwsProfile() (string, error) {
	homeDirPath, err := os.UserHomeDir()

	aws := filepath.Join(homeDirPath, ".aws")
	awsCredentialsPath := filepath.Join(aws, "credentials")

	_, err = os.Stat(aws)
	if os.IsNotExist(err) {
		err := os.MkdirAll(aws, 0755)
		if err != nil {
			return "", err
		}
	}

	var f *os.File

	_, err = os.Stat(awsCredentialsPath)
	if os.IsNotExist(err) {
		f, err = os.OpenFile(awsCredentialsPath, os.O_RDWR|os.O_CREATE, 0600)
		if err != nil {
			return awsCredentialsPath, fmt.Errorf("can't open file: %w", err)
		}
	} else {
		f, err = os.OpenFile(filepath.Join(awsCredentialsPath), os.O_RDWR|os.O_APPEND, 0600)
		if err != nil {
			return awsCredentialsPath, fmt.Errorf("can't open file: %w", err)
		}
	}

	defer func() {
		cerr := f.Close()
		if err == nil {
			err = cerr
		}
	}()

	ak := os.Getenv("AWS_ACCESS_KEY_ID")
	sk := os.Getenv("AWS_SECRET_ACCESS_KEY")
	r := viper.GetString("AWS_REGION")
	p := viper.GetString("AWS_PROFILE")
	if ak == "" || sk == "" || r == "" || p == "" {
		return awsCredentialsPath, fmt.Errorf("AWS_ACCESS_KEY_ID, AWS_SECRET_ACCESS_KEY, AWS_REGION, AWS_PROFILE must be set")
	}

	ls := ""
	localStackEnabled := viper.GetBool("LOCALSTACK")
	if localStackEnabled == true {
		localstackEndpoint := viper.GetString("LOCALSTACK_ENDPOINT")
		if localstackEndpoint == "" {
			localstackEndpoint = "http://127.0.0.1:4566"
		}
		ls = fmt.Sprintf("endpoint_url = %s", localstackEndpoint)
	}

	_, err = f.WriteString(fmt.Sprintf("[%v]\naws_access_key_id = %v\naws_secret_access_key = %v\nregion = %v\n%s\n\n", p, ak, sk, r, ls))
	if err != nil {
		return awsCredentialsPath, fmt.Errorf("can't write to %s", filepath.Join(awsCredentialsPath))
	}

	return awsCredentialsPath, nil
}
