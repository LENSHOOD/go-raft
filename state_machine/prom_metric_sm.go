package state_machine

import (
	"fmt"
	"github.com/LENSHOOD/go-raft/core"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/push"
	"strconv"
)

var (
	appliedCmd = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "raft_cmd_applied",
	})
)

type PromMetricStateMachine struct {
	pusher *push.Pusher
}

func NewPromMetricSm(pushGatewayUrl string, jobName string) *PromMetricStateMachine {
	pusher := push.New(pushGatewayUrl, jobName).Collector(appliedCmd)
	return &PromMetricStateMachine{
		pusher,
	}
}

func (p *PromMetricStateMachine) Exec(cmd core.Command) interface{} {
	cmdStr := fmt.Sprintf("%s", cmd)

	if v, err := strconv.Atoi(cmdStr); err == nil {
		// integer cmd
		appliedCmd.Set(float64(v))
	} else {
		return "Cmd Should Be Number Format Error: " + err.Error()
	}

	_ = p.pusher.Add()
	return "Cmd " + cmdStr + " Applied."
}
