package serverless

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/hazelops/ize/pkg/term"
	"github.com/sirupsen/logrus"
)

func (sls *Manager) runNpmInstall(w io.Writer) error {
	nvmDir := os.Getenv("NVM_DIR")
	if len(nvmDir) == 0 {
		nvmDir = "$HOME/.nvm"
	}

	command := fmt.Sprintf("source %s/nvm.sh && nvm use %s && npm install --save-dev", nvmDir, sls.App.NodeVersion)

	if sls.App.UseYarn {
		command = npmToYarn(command)
	}

	logrus.SetOutput(w)
	logrus.Debugf("command: %s", command)

	cmd := exec.Command("bash", "-c", command)
	cmd.Stdout = w
	cmd.Stderr = w
	cmd.Dir = filepath.Join(sls.App.Path)
	err := cmd.Run()
	if err != nil {
		return err
	}

	return nil
}

func (sls *Manager) nvm(w io.Writer, command string) error {

	nvmDir, err := sls.installNvm()
	if err != nil {
		return err
	}

	err = sls.readNvmrc()
	if err != nil {
		return err
	}

	logrus.Debugf("Running: bash -c source %s/nvm.sh && nvm install %s && %s", nvmDir, sls.App.NodeVersion, command)

	var bashFlags = "-c"
	// If log level is debug or trace, enable verbose mode for bash wrapper
	if sls.Project.LogLevel == "debug" || sls.Project.LogLevel == "trace" {
		bashFlags = "-xvc"
	}

	cmd := exec.Command("bash", bashFlags,
		fmt.Sprintf("source %s/nvm.sh && nvm install %s && %s", nvmDir, sls.App.NodeVersion, command),
	)

	// Capture stderr in a buffer
	var stderr bytes.Buffer
	var stdout bytes.Buffer
	cmd.Stderr = &stderr
	cmd.Stdout = &stdout

	t := term.New(
		term.WithDir(sls.App.Path),
		term.WithStdout(w),
		term.WithStderr(&stderr),
	)

	err = t.InteractiveRun(cmd)
	if err != nil {
		// Return the error along with stderr output
		return fmt.Errorf("command failed with error: %w, %s %s", err, stderr.String(), stdout.String())
	}

	return nil
}

func (sls *Manager) readNvmrc() error {
	_, err := os.Stat(filepath.Join(sls.App.Path, ".nvmrc"))
	if os.IsNotExist(err) {
	} else {
		file, err := os.ReadFile(filepath.Join(sls.App.Path, ".nvmrc"))
		if err != nil {
			return fmt.Errorf("can't read .nvmrc: %w", err)
		}
		sls.App.NodeVersion = strings.TrimSpace(string(file))
	}

	return nil
}

func (sls *Manager) runNvm(w io.Writer) error {
	nvmDir, err := sls.installNvm()
	if err != nil {
		return err
	}

	err = sls.readNvmrc()
	if err != nil {
		return err
	}

	command := fmt.Sprintf("source %s/nvm.sh && nvm install %s", nvmDir, sls.App.NodeVersion)

	logrus.SetOutput(w)
	logrus.Debugf("command: %s", command)

	cmd := exec.Command("bash", "-c", command)

	// Capture stderr in a buffer
	var stderr bytes.Buffer
	var stdout bytes.Buffer
	cmd.Stderr = &stderr
	cmd.Stdout = &stdout

	t := term.New(
		term.WithDir(sls.App.Path),
		term.WithStdout(w),
		term.WithStderr(&stderr),
	)

	err = t.InteractiveRun(cmd)
	if err != nil {
		// Return the error along with stderr output
		return fmt.Errorf("command failed with error: %w, %s %s", err, stderr.String(), stdout.String())
	}

	return nil
}

func (sls *Manager) runDeploy(w io.Writer) error {
	nvmDir, err := sls.installNvm()
	if err != nil {
		return err
	}
	var command string

	// SLS v3 has breaking changes in syntax
	if sls.App.ServerlessVersion == "3" {
		command = fmt.Sprintf(
			`source %s/nvm.sh && 
				nvm use %s &&
				npx serverless deploy \
				--config=%s \
				--param="service=%s" \
				--region=%s \
				--aws-profile=%s \
				--stage=%s \
				--verbose`,
			nvmDir, sls.App.NodeVersion, sls.App.File,
			sls.App.Name, sls.App.AwsRegion,
			sls.App.AwsProfile, sls.Project.Env)
	} else {
		command = fmt.Sprintf(
			`source %s/nvm.sh && 
				nvm use %s &&
				npx serverless deploy \
				--config %s \
				--service %s \
				--verbose \
				--region %s \
				--aws-profile %s \
				--stage %s`,
			nvmDir, sls.App.NodeVersion, sls.App.File,
			sls.App.Name, sls.App.AwsRegion,
			sls.App.AwsProfile, sls.Project.Env)
	}

	if sls.App.UseYarn {
		command = npmToYarn(command)
	}

	if sls.App.Force {
		command += ` \
		--force`
	}

	logrus.SetOutput(w)
	logrus.Debugf("command: %s", command)

	cmd := exec.Command("bash", "-c", command)

	// Capture stderr in a buffer
	var stderr bytes.Buffer
	var stdout bytes.Buffer
	cmd.Stderr = &stderr
	cmd.Stdout = &stdout

	t := term.New(
		term.WithDir(sls.App.Path),
		term.WithStdout(w),
		term.WithStderr(&stderr),
	)

	err = t.InteractiveRun(cmd)
	if err != nil {
		// Return the error along with stderr output
		return fmt.Errorf("command failed with error: %w, %s %s", err, stderr.String(), stdout.String())
	}

	return nil
}

func (sls *Manager) runRemove(w io.Writer) error {

	nvmDir, err := sls.installNvm()
	if err != nil {
		return err
	}

	var command string

	// SLS v3 has breaking changes in syntax
	if sls.App.ServerlessVersion == "3" {
		command = fmt.Sprintf(
			`source %s/nvm.sh && \
				nvm use %s && \
				npx serverless remove \
				--config=%s \
				--param="service=%s" \
				--region=%s \
				--aws-profile=%s \
				--stage=%s \
				--verbose`,
			nvmDir, sls.App.NodeVersion, sls.App.File,
			sls.App.Name, sls.App.AwsRegion,
			sls.App.AwsProfile, sls.Project.Env)
	} else {
		command = fmt.Sprintf(
			`source %s/nvm.sh && \
				nvm use %s && \
				npx serverless remove \
				--config %s \
				--service %s \
				--verbose \
				--region %s \
				--aws-profile %s \
				--stage %s`,
			nvmDir, sls.App.NodeVersion, sls.App.File,
			sls.App.Name, sls.App.AwsRegion,
			sls.App.AwsProfile, sls.Project.Env)
	}

	if sls.App.UseYarn {
		command = npmToYarn(command)
	}

	logrus.SetOutput(w)
	logrus.Debugf("command: %s", command)

	cmd := exec.Command("bash", "-c", command)

	// Capture stderr in a buffer
	var stderr bytes.Buffer
	var stdout bytes.Buffer
	cmd.Stderr = &stderr
	cmd.Stdout = &stdout

	t := term.New(
		term.WithDir(sls.App.Path),
		term.WithStdout(w),
		term.WithStderr(&stderr),
	)

	err = t.InteractiveRun(cmd)
	if err != nil {
		// Return the error along with stderr output
		return fmt.Errorf("command failed with error: %w, %s %s", err, stderr.String(), stdout.String())
	}

	return nil
}

func (sls *Manager) runCreateDomain(w io.Writer) error {
	nvmDir, err := sls.installNvm()
	if err != nil {
		return err
	}

	command := fmt.Sprintf(
		`source %s/nvm.sh && \
				nvm use %s && \
				npx serverless create_domain \
				--verbose \
				--region %s \
				--aws-profile %s \
				--stage %s`,
		nvmDir, sls.App.NodeVersion, sls.App.AwsRegion,
		sls.App.AwsProfile, sls.Project.Env)

	if sls.App.UseYarn {
		command = npmToYarn(command)
	}

	logrus.SetOutput(w)
	logrus.Debugf("command: %s", command)

	cmd := exec.Command("bash", "-c", command)

	// Capture stderr in a buffer
	var stderr bytes.Buffer
	var stdout bytes.Buffer
	cmd.Stderr = &stderr
	cmd.Stdout = &stdout

	t := term.New(
		term.WithDir(sls.App.Path),
		term.WithStdout(w),
		term.WithStderr(&stderr),
	)

	err = t.InteractiveRun(cmd)
	if err != nil {
		// Return the error along with stderr output
		return fmt.Errorf("command failed with error: %w, %s %s", err, stderr.String(), stdout.String())
	}

	return nil
}

func (sls *Manager) runRemoveDomain(w io.Writer) error {
	nvmDir, err := sls.installNvm()
	if err != nil {
		return err
	}

	command := fmt.Sprintf(
		`source %s/nvm.sh && \
				nvm use %s && \
				npx serverless delete_domain \
				--verbose \
				--region %s \
				--aws-profile %s \
				--stage %s`,
		nvmDir, sls.App.NodeVersion, sls.App.AwsRegion,
		sls.App.AwsProfile, sls.Project.Env)

	if sls.App.UseYarn {
		command = npmToYarn(command)
	}

	logrus.SetOutput(w)
	logrus.Debugf("command: %s", command)

	cmd := exec.Command("bash", "-c", command)

	// Capture stderr in a buffer
	var stderr bytes.Buffer
	var stdout bytes.Buffer
	cmd.Stderr = &stderr
	cmd.Stdout = &stdout

	t := term.New(
		term.WithDir(sls.App.Path),
		term.WithStdout(w),
		term.WithStderr(&stderr),
	)

	err = t.InteractiveRun(cmd)
	if err != nil {
		// Return the error along with stderr output
		return fmt.Errorf("command failed with error: %w, %s %s", err, stderr.String(), stdout.String())
	}

	return nil
}

func npmToYarn(cmd string) string {
	cmd = strings.ReplaceAll(cmd, "npm", "yarn")
	return strings.ReplaceAll(cmd, "npx", "yarn")
}

func (sls *Manager) installNvm() (string, error) {
	var err error

	nvmDir := os.Getenv("NVM_DIR")
	if len(nvmDir) == 0 {
		nvmDir = "$HOME/.nvm"
	}

	// Check if nvm.sh exists in the nvmDir
	nvmShPath := filepath.Join(nvmDir, "nvm.sh")
	_, err = os.Stat(nvmShPath)
	if !os.IsNotExist(err) {
		logrus.Debug("nvm.sh found in the directory:", nvmDir)

		//check if version is what we expect
		cmd := exec.Command("bash", "-c", fmt.Sprintf("source %s/nvm.sh && nvm --version", nvmDir))
		cmd.Dir = sls.Project.RootDir

		var out bytes.Buffer
		cmd.Stdout = &out
		err = cmd.Run()
		if err != nil {
			logrus.Debug("Error checking nvm version:", err)
			return "", err
		}

		if strings.TrimSpace(out.String()) == sls.Project.NvmVersion {
			logrus.Debugf("nvm version is correct. Expected: %s, Found: %s", sls.Project.NvmVersion, strings.TrimSpace(out.String()))
			return nvmDir, nil
		}

	}
	logrus.Debug("No correct nvm version found is incorrect, (re)installing nvm")

	// Install nvm.
	cmd := exec.Command("bash", "-c", fmt.Sprintf("curl -o- https://raw.githubusercontent.com/nvm-sh/nvm/v%s/install.sh | bash", sls.Project.NvmVersion))
	cmd.Dir = sls.Project.RootDir
	err = cmd.Run()
	if err != nil {
		logrus.Debugf("Error installing nvm:", err)
		return "", err
	}

	// TODO: If nvm.sh doesn't exist in the nvmDir, we should install it
	// curl -o- https://raw.githubusercontent.com/nvm-sh/nvm/v0.40.0/install.sh | bash

	return nvmDir, nil
}
