//go:build chaos
// +build chaos

package state_machine

import (
	"fmt"
	"github.com/LENSHOOD/go-raft/comm"
	"github.com/LENSHOOD/go-raft/core"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/push"
	"os"
	"strconv"
)

var (
	instanceLabelKey      = "instance"
	instanceLabelValue, _ = os.Hostname()
	appliedCmd            = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "raft_cmd_applied",
	})
)

type PromMetricStateMachine struct {
	pusher *push.Pusher
}

func newPromMetricSm(pushGatewayUrl string, jobName string) *PromMetricStateMachine {
	pusher := push.New(pushGatewayUrl, jobName).Grouping(instanceLabelKey, instanceLabelValue).Collector(appliedCmd)
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
		comm.GetLogger().Errorf("[Prom SM] error to push metric: %v", err)
		return "Cmd Should Be Number Format Error: " + err.Error()
	}

	if err := p.pusher.Add(); err != nil {
		comm.GetLogger().Errorf("[Prom SM] error to push metric: %v", err)
	}
	return "Cmd " + cmdStr + " Applied."
}

var (
	url = "push-gateway-svc.prometheus:9091"
	job = "raft-instance"
)

func BuildStateMachine() core.StateMachine {
	return newPromMetricSm(url, job)
}
