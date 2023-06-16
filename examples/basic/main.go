package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/goforbroke1006/asyncqu"
)

func main() {
	executor := asyncqu.New()

	const (
		LabelStep1LoadData    = asyncqu.StepName("phase-1-load-data")
		LabelStep2FilterData  = asyncqu.StepName("phase-2-filter-data")
		LabelStep3Aggregate1  = asyncqu.StepName("phase-3-aggregate-1")
		LabelStep3Aggregate2  = asyncqu.StepName("phase-3-aggregate-2")
		LabelStep3Aggregate3  = asyncqu.StepName("phase-3-aggregate-3")
		LabelStep4Additional1 = asyncqu.StepName("phase-4-additional-1")
	)

	executor.SetOnChanges(func(name asyncqu.StepName, state asyncqu.JobState) {
		fmt.Printf("step %s in status %s\n", name, state)
	})

	executor.Append(LabelStep1LoadData, func(ctx context.Context) error {
		time.Sleep(time.Second)
		return nil
	}, asyncqu.Start)

	executor.Append(LabelStep2FilterData, func(ctx context.Context) error {
		time.Sleep(time.Second)
		return nil
	}, LabelStep1LoadData)

	executor.Append(LabelStep3Aggregate1, func(ctx context.Context) error {
		time.Sleep(time.Second)
		return nil
	}, LabelStep2FilterData)
	executor.Append(LabelStep3Aggregate2, func(ctx context.Context) error {
		time.Sleep(time.Second)
		return nil
	}, LabelStep2FilterData)
	executor.Append(LabelStep3Aggregate3, func(ctx context.Context) error {
		time.Sleep(time.Second)
		return nil
	}, LabelStep2FilterData)

	executor.Append(LabelStep4Additional1, func(ctx context.Context) error {
		time.Sleep(time.Second)
		return nil
	}, LabelStep3Aggregate3)

	executor.AddEnd(LabelStep3Aggregate1, LabelStep3Aggregate2, LabelStep4Additional1)

	if runErr := executor.AsyncRun(context.Background()); runErr != nil {
		fmt.Printf("ERROR: %s\n", runErr.Error())
		os.Exit(1)
	}

	_ = executor.Wait()

	if execErrs := executor.Errs(); len(execErrs) > 0 {
		for _, err := range execErrs {
			fmt.Printf("ERROR: %s\n", err.Error())
		}
	}
}
