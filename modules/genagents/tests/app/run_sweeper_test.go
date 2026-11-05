package app_test

import (
	"context"
	"testing"
	"time"

	"github.com/JoaoBittencourt1/Genwork/modules/genagents/internal/runexec"
	"github.com/JoaoBittencourt1/Genwork/modules/genagents/internal/runs"
)

func (app *testApp) sweep() int {
	app.t.Helper()
	ended, err := runexec.NewRunSweeper(app.db, runs.NewRunLifecycleService(app.clock.Now), app.clock.Now).SweepOnce(context.Background())
	if err != nil {
		app.t.Fatalf("sweep: %v", err)
	}
	return ended
}

func TestSweeperFailsRunsWhoseRunnerWentSilent(t *testing.T) {
	app := newTestApp(t)
	runner, claimed, _ := app.claimedRunOf(runs.RunKindTask, nil)

	app.clock.Advance(4 * time.Minute)
	if app.sweep() != 0 || app.runState(claimed.RunToken) != "running" {
		t.Fatal("a run was ended before its runner was gone for 5 minutes")
	}
	expectStatus(t, app.sendHeartbeat(runner, heartbeatOptions{tools: []string{"claude"}, github: true}), 200)
	app.clock.Advance(4 * time.Minute)
	if app.sweep() != 0 {
		t.Fatal("a heartbeat did not keep the run alive")
	}
	app.clock.Advance(2 * time.Minute)

	if ended := app.sweep(); ended != 1 || app.runState(claimed.RunToken) != "failed" || app.runFailureReason(claimed.RunToken) != "runner_lost" {
		t.Errorf("ended = %d, state = %s (%s)", ended, app.runState(claimed.RunToken), app.runFailureReason(claimed.RunToken))
	}
	last := app.conversationEntries(claimed.RunToken)
	if last[len(last)-1]["_type"] != "error" || last[len(last)-1]["code"] != "run_failed" || last[len(last)-1]["reason"] != "runner_lost" {
		t.Errorf("last entry = %v", last[len(last)-1])
	}
}

func TestSweeperFailsRunsPastTheirTimeout(t *testing.T) {
	app := newTestApp(t)
	runner, claimed, _ := app.claimedRunOf(runs.RunKindTask, func(body map[string]any) { body["timeoutMinutes"] = 10 })

	for minute := 0; minute < 11; minute++ {
		app.clock.Advance(time.Minute)
		expectStatus(t, app.sendHeartbeat(runner, heartbeatOptions{tools: []string{"claude"}, github: true}), 200)
	}

	if app.sweep() != 1 || app.runFailureReason(claimed.RunToken) != "timeout" {
		t.Errorf("state = %s (%s)", app.runState(claimed.RunToken), app.runFailureReason(claimed.RunToken))
	}
}

func TestSweeperFailsRunsWaitingTooLongForAReply(t *testing.T) {
	app := newTestApp(t)
	runner, claimed, _ := app.claimedRunOf(runs.RunKindTask, func(body map[string]any) { body["timeoutMinutes"] = 720 })
	app.postEvents(runner, claimed.RunToken, eventBody{Sequence: 1, Type: "status", Payload: map[string]any{"state": "waiting_for_input"}})

	for hour := 0; hour < 23; hour++ {
		app.clock.Advance(time.Hour)
		expectStatus(t, app.sendHeartbeat(runner, heartbeatOptions{tools: []string{"claude"}, github: true}), 200)
	}
	if app.sweep() != 0 {
		t.Fatal("a waiting run ended before 24 hours")
	}
	app.clock.Advance(time.Hour)
	expectStatus(t, app.sendHeartbeat(runner, heartbeatOptions{tools: []string{"claude"}, github: true}), 200)

	if app.sweep() != 1 || app.runFailureReason(claimed.RunToken) != "no_reply" {
		t.Errorf("state = %s (%s)", app.runState(claimed.RunToken), app.runFailureReason(claimed.RunToken))
	}
}

func TestSweeperLeavesHealthyRunsAlone(t *testing.T) {
	app := newTestApp(t)
	_, claimed, _ := app.claimedRunOf(runs.RunKindTask, nil)

	app.clock.Advance(30 * time.Second)

	if app.sweep() != 0 || app.runState(claimed.RunToken) != "running" {
		t.Errorf("state = %s", app.runState(claimed.RunToken))
	}
}
