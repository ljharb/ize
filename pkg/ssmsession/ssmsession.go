package ssmsession

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"

	"github.com/aws/aws-sdk-go/service/ecs"
	"github.com/hazelops/ize/pkg/term"
)

const (
	ssmPluginBinaryName = "session-manager-plugin"
	startSessionAction  = "StartSession"
)

type SSMPluginRunner interface {
	InteractiveRun(cmd *exec.Cmd) (stdout string, stderr string, exitCode int, err error)
}

type SSMPluginCommand struct {
	runner SSMPluginRunner
	region string
}

func NewSSMPluginCommand(region string) SSMPluginCommand {
	return SSMPluginCommand{
		runner: term.New(),
		region: region,
	}
}

func (s SSMPluginCommand) Start(ssmSession *ecs.Session) error {
	response, err := json.Marshal(ssmSession)
	if err != nil {
		return fmt.Errorf("marshal session response: %w", err)
	}
	cmd := exec.Command(ssmPluginBinaryName, []string{string(response), s.region, startSessionAction}...)
	out, _, _, err := s.runner.InteractiveRun(cmd)
	if err != nil {
		return fmt.Errorf("start session: %w", err)
	}
	if strings.Contains(out, "ERROR") {
		return fmt.Errorf("exit status: 1")
	}

	return nil
}
