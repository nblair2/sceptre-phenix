package scheduler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"

	ifaces "phenix/types/interfaces"
	"phenix/util/common"
	"phenix/util/mm"
	"phenix/util/plog"
	"phenix/util/shell"
)

var ErrUserSchedulerNotFound = errors.New("user scheduler not found")

type userScheduler struct {
	options Options
}

func (this *userScheduler) Init(opts ...Option) error {
	this.options = NewOptions(opts...)

	return nil
}

func (this userScheduler) Name() string {
	return this.options.Name
}

func (this userScheduler) Schedule(spec ifaces.ExperimentSpec) error {
	if err := this.shellOut(spec); err != nil {
		return fmt.Errorf("running user scheduler: %w", err)
	}

	return nil
}

func (this userScheduler) shellOut(spec ifaces.ExperimentSpec) error {
	cmdName := "phenix-scheduler-" + this.options.Name

	if !shell.CommandExists(cmdName) {
		return fmt.Errorf("external user scheduler %s does not exist in your path: %w", cmdName, ErrUserSchedulerNotFound)
	}

	cluster, err := mm.GetClusterHosts(true)
	if err != nil {
		return fmt.Errorf("getting cluster hosts: %w", err)
	}

	exp := struct {
		Spec  ifaces.ExperimentSpec `json:"spec"`
		Hosts mm.Hosts              `json:"hosts"`
	}{
		Spec:  spec,
		Hosts: cluster,
	}

	data, err := json.Marshal(exp)
	if err != nil {
		return fmt.Errorf("marshaling experiment spec to JSON: %w", err)
	}

	logPipePath := filepath.Join(common.PhenixBase, "experiments", spec.ExperimentName(), "scheduler_pipes", this.options.Name)
	done, err := plog.ReadProcessLogs(logPipePath, plog.TypeSystem, "scheduler", this.options.Name, "exp", spec.ExperimentName())
	defer close(done)
	if err != nil {
		return err
	}

	opts := []shell.Option{
		shell.Command(cmdName),
		shell.Stdin(data),
		shell.Env( // TODO: update to reflect options provided by user
			"PHENIX_LOG_LEVEL=DEBUG",
			"PHENIX_LOG_FILE="+logPipePath,
			"PHENIX_DIR="+common.PhenixBase,
		),
	}

	stdOut, stdErr, err := shell.ExecCommand(context.Background(), opts...)
	if err != nil {
		plog.Warn(plog.TypeSystem, "scheduler returned stderr", "stderr", string(stdErr), "scheduler", this.options.Name, "exp", exp)


		return fmt.Errorf("user scheduler %s command %s failed: %w", this.options.Name, cmdName, err)
	}

	if err := json.Unmarshal(stdOut, &exp); err != nil {
		return fmt.Errorf("unmarshaling experiment spec from JSON: %w", err)
	}

	spec.SetSchedule(exp.Spec.Schedules())

	return nil
}
