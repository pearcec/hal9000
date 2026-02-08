package helloworld

import (
	"context"
	"fmt"
	"time"

	"github.com/pearcec/hal9000/cmd/hal9000/tasks"
)

func init() {
	tasks.Register(&HelloWorldTask{})
}

// HelloWorldTask is a simple demo task for scheduler testing.
type HelloWorldTask struct{}

func (t *HelloWorldTask) Name() string            { return "helloworld" }
func (t *HelloWorldTask) Description() string      { return "Hello World demo task" }
func (t *HelloWorldTask) PreferencesKey() string   { return "" }
func (t *HelloWorldTask) SetupQuestions() []tasks.SetupQuestion { return nil }

func (t *HelloWorldTask) Run(ctx context.Context, opts tasks.RunOptions) (*tasks.Result, error) {
	msg := fmt.Sprintf("Hello, World! The time is %s.", time.Now().Format("3:04:05 PM"))
	return &tasks.Result{
		Success: true,
		Output:  msg,
		Message: msg,
	}, nil
}
