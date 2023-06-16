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
		stage1LoadData    = asyncqu.StageName("stage-1-load-data")
		stage2FilterData  = asyncqu.StageName("stage-2-filter-data")
		stage3Aggregate1  = asyncqu.StageName("stage-3-aggregate-1")
		stage3Aggregate2  = asyncqu.StageName("stage-3-aggregate-2")
		stage3Aggregate3  = asyncqu.StageName("stage-3-aggregate-3")
		stage4Additional1 = asyncqu.StageName("stage-4-additional-1")
	)

	executor.SetOnChanges(func(stageName asyncqu.StageName, state asyncqu.State, err error) {
		fmt.Printf("step %s in status %s with %v error\n", stageName, state, err)
	})

	executor.Append(stage1LoadData, func(ctx context.Context) error {
		time.Sleep(time.Second)
		return nil
	}, asyncqu.Start)

	executor.Append(stage2FilterData, func(ctx context.Context) error {
		time.Sleep(time.Second)
		return nil
	}, stage1LoadData)

	executor.Append(stage3Aggregate1, func(ctx context.Context) error {
		time.Sleep(time.Second)
		return nil
	}, stage2FilterData)
	executor.Append(stage3Aggregate2, func(ctx context.Context) error {
		time.Sleep(time.Second)
		return nil
	}, stage2FilterData)
	executor.Append(stage3Aggregate3, func(ctx context.Context) error {
		time.Sleep(time.Second)
		return nil
	}, stage2FilterData)

	executor.Append(stage4Additional1, func(ctx context.Context) error {
		time.Sleep(time.Second)
		return nil
	}, stage3Aggregate3)

	executor.SetEnd(stage3Aggregate1, stage3Aggregate2, stage4Additional1)

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
