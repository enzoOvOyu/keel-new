package flows

import (
	"context"
	"regexp"
	"strings"
)

type JobStep struct {
	ID        string            `json:"id"`
	Kind      string            `json:"kind"`
	Command   string            `json:"command"`
	DependsOn []string          `json:"depends_on"`
	Inputs    map[string]string `json:"inputs"` // environment variable -> predecessor step ID
}
type StepResult struct {
	ID     string `json:"id"`
	State  string `json:"state"`
	Exit   int    `json:"exit_code"`
	Stdout string `json:"stdout"`
	Stderr string `json:"stderr"`
}

var envName = regexp.MustCompile(`^[A-Z][A-Z0-9_]{0,63}$`)

func PlanSteps(steps []JobStep) ([]JobStep, error) {
	if len(steps) > 32 {
		return nil, problem(422, "TOO_MANY_JOB_STEPS")
	}
	remaining := map[string]JobStep{}
	ordered := []JobStep{}
	done := map[string]bool{}
	for _, step := range steps {
		if !identifier.MatchString(step.ID) || remaining[step.ID].ID != "" || step.Command == "" || len(step.Command) > 131072 || !contains([]string{"command", "bash", "python"}, step.Kind) || strings.Contains(step.Command, "PRIVATE KEY-----") {
			return nil, problem(422, "JOB_STEP_INVALID")
		}
		if e := ValidateTaskParameters(step.Kind, step.Command); e != nil {
			return nil, e
		}
		remaining[step.ID] = step
	}
	for len(remaining) > 0 {
		progress := false
		for _, step := range steps {
			if _, ok := remaining[step.ID]; !ok {
				continue
			}
			ready := true
			for _, d := range step.DependsOn {
				if !done[d] {
					ready = false
				}
			}
			for name, source := range step.Inputs {
				if !envName.MatchString(name) || !contains(step.DependsOn, source) {
					return nil, problem(422, "STEP_INPUT_MUST_REFERENCE_DEPENDENCY")
				}
			}
			if ready {
				ordered = append(ordered, step)
				done[step.ID] = true
				delete(remaining, step.ID)
				progress = true
			}
		}
		if !progress {
			return nil, problem(422, "STEP_DEPENDENCY_CYCLE_OR_MISSING")
		}
	}
	return ordered, nil
}
func StepTask(root Task, step JobStep, values map[string]StepResult) Task {
	env := map[string]string{}
	for key, source := range step.Inputs {
		env[key] = values[source].Stdout
	}
	task := Task{ID: root.ID + ":" + step.ID, Kind: step.Kind, Command: step.Command, Timeout: root.Timeout, Env: env}
	task.SHA256 = TaskDigest(task)
	return task
}
func RunSSHPipeline(ctx context.Context, host string, port int, user, key, known string, task Task, progress ...func(StepResult)) (Result, error) {
	if len(task.Steps) == 0 {
		return RunSSH(ctx, host, port, user, key, known, task)
	}
	plan, e := PlanSteps(task.Steps)
	if e != nil {
		return Result{}, e
	}
	out := Result{ID: task.ID, State: "SUCCESS", Steps: []StepResult{}}
	values := map[string]StepResult{}
	for _, step := range plan {
		if out.State != "SUCCESS" {
			out.Steps = append(out.Steps, StepResult{ID: step.ID, State: "BLOCKED", Exit: -1})
			continue
		}
		if len(progress) > 0 {
			progress[0](StepResult{ID: step.ID, State: "RUNNING", Exit: -1})
		}
		r, e := RunSSH(ctx, host, port, user, key, known, StepTask(task, step, values))
		if e != nil {
			return out, e
		}
		v := StepResult{ID: step.ID, State: r.State, Exit: r.Exit, Stdout: BoundStepOutput(r.Stdout), Stderr: BoundStepOutput(r.Stderr)}
		if len(progress) > 0 {
			progress[0](v)
		}
		out.Steps = append(out.Steps, v)
		values[step.ID] = v
		out.Stdout += v.Stdout
		out.Stderr += v.Stderr
		out.State = r.State
		out.Exit = r.Exit
	}
	return out, nil
}
func (s *Service) SaveStepResults(ctx context.Context, execution, asset string, values []StepResult) error {
	for _, v := range values {
		if _, e := s.DB.Exec(ctx, `INSERT INTO flow_step_result(execution_id,asset_id,step_id,state,exit_code,stdout,stderr) VALUES($1,$2,$3,$4,$5,$6,$7) ON CONFLICT(execution_id,asset_id,step_id) DO UPDATE SET state=excluded.state,exit_code=excluded.exit_code,stdout=excluded.stdout,stderr=excluded.stderr`, execution, asset, v.ID, v.State, v.Exit, redactOutput(v.Stdout), redactOutput(v.Stderr)); e != nil {
			return e
		}
	}
	return nil
}

func TaskDigest(t Task) string {
	if len(t.Steps) == 0 && len(t.Env) == 0 {
		return hash(t.Command)
	}
	return hash(string(raw(struct {
		Command string
		Steps   []JobStep
		Env     map[string]string
	}{t.Command, t.Steps, t.Env})))
}

// Keep pipeline result messages bounded independently of script stdout buffers.
func BoundStepOutput(value string) string {
	if len(value) <= 32768 {
		return value
	}
	return value[:32768] + "\n[step output truncated at 32KiB]"
}
