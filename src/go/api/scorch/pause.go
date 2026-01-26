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

type UniformMetadata struct {
	Minimum *time.Duration `mapstructure:"minimum"`
	Maximum *time.Duration `mapstructure:"maximum"`
}

type GaussianMetadata struct {
	Mean   *time.Duration `mapstructure:"mean"`
	StdDev *time.Duration `mapstructure:"stddev"`
}

type ExponentialMetadata struct {
	Mean *time.Duration `mapstructure:"mean"`
}

type RandomMetadata struct {
	Uniform     *UniformMetadata     `mapstructure:"uniform"`
	Gaussian    *GaussianMetadata    `mapstructure:"gaussian"`
	Exponential *ExponentialMetadata `mapstructure:"exponential"`
}

func (r *RandomMetadata) Generate() (time.Duration, error) {
	// Check for multiple distributions defined
	count := 0
	if r.Uniform != nil {
		count++
	}
	if r.Gaussian != nil {
		count++
	}
	if r.Exponential != nil {
		count++
	}
	if count > 1 {
		return 0, fmt.Errorf("cannot specify multiple distributions (uniform, gaussian, exponential)")
	}

	if r.Gaussian != nil {
		mean := 10 * time.Second
		stddev := 2 * time.Second
		if r.Gaussian.Mean != nil {
			mean = *r.Gaussian.Mean
		}
		if r.Gaussian.StdDev != nil {
			stddev = *r.Gaussian.StdDev
		}

		val := rand.NormFloat64()*float64(stddev) + float64(mean)
		if val < 0 {
			val = 0
		}
		return time.Duration(val), nil
	}

	if r.Exponential != nil {
		mean := 10 * time.Second
		if r.Exponential.Mean != nil {
			mean = *r.Exponential.Mean
		}
		return time.Duration(rand.ExpFloat64() * float64(mean)), nil
	}

	// Default to Uniform
	min := time.Duration(0)
	max := 10 * time.Second
	if r.Uniform != nil {
		if r.Uniform.Minimum != nil {
			min = *r.Uniform.Minimum
		}
		if r.Uniform.Maximum != nil {
			max = *r.Uniform.Maximum
		}
	}

	if max <= min {
		return 0, fmt.Errorf("maximum must be greater than minimum")
	}

	return min + time.Duration(rand.Float64()*float64(max-min)), nil
}

type PauseMetadata struct {
	Duration   *time.Duration  `mapstructure:"duration"`
	Random     *RandomMetadata `mapstructure:"random"`
	FailStages []string        `mapstructure:"failStages"`
}

func (this *PauseMetadata) Validate() error {
	if this.Duration != nil && this.Random != nil {
		return fmt.Errorf("cannot specify both duration and random")
	}

	if this.Duration != nil {
		return nil
	}

	if this.Random != nil {
		d, err := this.Random.Generate()
		if err != nil {
			return err
		}
		this.Duration = &d
		return nil
	}

	d := 10 * time.Second
	this.Duration = &d
	return nil
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
