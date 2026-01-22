package scorch

import (
	"context"
	"fmt"
	"math/rand/v2"
	"time"

	"phenix/util"
	"phenix/web/scorch"

	"github.com/fatih/color"
	"github.com/mitchellh/mapstructure"
)

type PauseMetadata struct {
	Duration   *time.Duration `mapstructure:"duration"`
	Minimum    *time.Duration `mapstructure:"minimum"`
	Maximum    *time.Duration `mapstructure:"maximum"`
	FailStages []string       `mapstructure:"failStages"`
}

func (this *PauseMetadata) Validate() error {
	if this.Maximum != nil {
		if this.Duration != nil {
			return fmt.Errorf("cannot specify both duration and maximum")
		}

		minimum := time.Duration(0)
		if this.Minimum != nil {
			minimum = *this.Minimum
		}

		diff := *this.Maximum - minimum
		if diff <= 0 {
			return fmt.Errorf("maximum must be greater than minimum")
		}

		d := minimum + time.Duration(rand.Int64N(int64(diff)))
		this.Duration = &d
		return nil
	}

	if this.Duration == nil {
		if this.Minimum != nil {
			return fmt.Errorf("cannot specify both duration and minimum")
		}
		d := 10 * time.Second
		this.Duration = &d
		return nil
	}

	return fmt.Errorf("must specify duration or maximum. If maximum is specified, minimum is optional")
}

type Pause struct {
	options Options
}

func (this *Pause) Init(opts ...Option) error {
	this.options = NewOptions(opts...)
	return nil
}

func (this Pause) Type() string {
	return "pause"
}

func (this Pause) Configure(ctx context.Context) error {
	if this.options.Background {
		ctx = background(ctx, ACTIONCONFIG, this.options)
		go this.pause(ctx, ACTIONCONFIG)
		return nil
	}

	return this.pause(ctx, ACTIONCONFIG)
}

func (this Pause) Start(ctx context.Context) error {
	if this.options.Background {
		ctx = background(ctx, ACTIONSTART, this.options)
		go this.pause(ctx, ACTIONSTART)
		return nil
	}

	return this.pause(ctx, ACTIONSTART)
}

func (this Pause) Stop(ctx context.Context) error {
	if handleBackgrounded(ACTIONSTOP, this.options) {
		return nil
	}

	return this.pause(ctx, ACTIONSTOP)
}

func (this Pause) Cleanup(ctx context.Context) error {
	if handleBackgrounded(ACTIONCLEANUP, this.options) {
		return nil
	}

	return this.pause(ctx, ACTIONCLEANUP)
}

func (this Pause) pause(ctx context.Context, stage Action) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}

	var md PauseMetadata

	decoder, err := mapstructure.NewDecoder(&mapstructure.DecoderConfig{
		DecodeHook: mapstructure.StringToTimeDurationHookFunc(),
		Result:     &md,
	})
	if err != nil {
		return fmt.Errorf("creating decoder: %w", err)
	}

	if err := decoder.Decode(this.options.Meta); err != nil {
		return fmt.Errorf("decoding pause component metadata: %w", err)
	}

	if err := md.Validate(); err != nil {
		return fmt.Errorf("validating pause component metadata: %w", err)
	}

	printer := color.New(color.FgYellow)
	printer.Printf("pausing for %v\n", *md.Duration)

	update := scorch.ComponentUpdate{
		Exp:     this.options.Exp.Spec.ExperimentName(),
		CmpName: this.options.Name,
		CmpType: this.options.Type,
		Run:     this.options.Run,
		Loop:    this.options.Loop,
		Count:   this.options.Count,
		Stage:   string(stage),
		Status:  "running",
	}

	start := time.Now()

	i := 1
	for time.Since(start) < *md.Duration {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(1 * time.Second):
			update.Output = []byte(fmt.Sprintf("pausing... (%ds / %v)\n", i, *md.Duration))
			scorch.UpdateComponent(update)
			i++
		}
	}

	if util.StringSliceContains(md.FailStages, string(stage)) {
		return fmt.Errorf("failing as instructed")
	}

	return nil
}
