package cpumem

import (
	"testing"

	resourcetypes "github.com/projecteru2/core/resources/types"
	"github.com/projecteru2/core/scheduler"
	schedulerMocks "github.com/projecteru2/core/scheduler/mocks"
	"github.com/projecteru2/core/types"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestMakeRequest(t *testing.T) {
	// Mem request below zero shall fail
	_, err := MakeRequest(types.ResourceOptions{
		MemoryRequest: -1,
		MemoryLimit:   -1,
	})
	assert.NotNil(t, err)

	// Mem and cpu request equal to zero will not fail
	_, err = MakeRequest(types.ResourceOptions{
		MemoryRequest:   0,
		MemoryLimit:     1,
		CPUQuotaRequest: 0,
		CPUQuotaLimit:   1,
	})
	assert.Nil(t, err)

	// Request more then limited will not fail
	_, err = MakeRequest(types.ResourceOptions{
		MemoryRequest:   2,
		MemoryLimit:     1,
		CPUQuotaRequest: 2,
		CPUQuotaLimit:   1,
	})
	assert.Nil(t, err)

	// Request below zero will fail
	_, err = MakeRequest(types.ResourceOptions{
		CPUQuotaRequest: -0.5,
		CPUQuotaLimit:   -1,
	})
	assert.NotNil(t, err)

	// Request unlimited cpu but with cpu bind will fail
	_, err = MakeRequest(types.ResourceOptions{
		CPUQuotaRequest: 0,
		CPUBind:         true,
	})
	assert.NotNil(t, err)
}

func TestType(t *testing.T) {
	req, err := MakeRequest(types.ResourceOptions{
		MemoryRequest:   0,
		MemoryLimit:     1,
		CPUQuotaRequest: 0,
		CPUQuotaLimit:   1,
	})
	assert.Nil(t, err)
	assert.True(t, req.Type()&(types.ResourceCPU|types.ResourceMemory) > 0)

	req, err = MakeRequest(types.ResourceOptions{
		CPUQuotaRequest: 1,
		CPUQuotaLimit:   1,
		CPUBind:         true,
	})
	assert.Nil(t, err)
	assert.True(t, req.Type()&types.ResourceCPUBind > 0)
}

func TestRequestCpuNode(t *testing.T) {
	run(t, newRequestCPUNodeTest())
}

func TestRequestMemNode(t *testing.T) {
	run(t, newRequestMemNodeTest(types.ResourceOptions{
		CPUQuotaRequest: 0.5,
		CPUQuotaLimit:   1,
		CPUBind:         false,
		MemoryRequest:   512,
		MemoryLimit:     1024,
	}))
	run(t, newRequestMemNodeTest(types.ResourceOptions{
		CPUQuotaRequest: 0,
		CPUQuotaLimit:   0,
		CPUBind:         false,
		MemoryRequest:   512,
		MemoryLimit:     1024,
	}))
}

type nodeSchdulerTest interface {
	getNodeInfos() []types.NodeInfo
	getScheduler() scheduler.Scheduler
	getRequestOptions() types.ResourceOptions
	getNode() *types.Node
	assertAfterChanges(t *testing.T)
	assertAfterRollback(t *testing.T)
}

func run(t *testing.T, test nodeSchdulerTest) {
	s := test.getScheduler()
	prevSche, _ := scheduler.GetSchedulerV1()
	scheduler.InitSchedulerV1(s)
	defer func() {
		scheduler.InitSchedulerV1(prevSche)
	}()

	resourceRequest, err := MakeRequest(test.getRequestOptions())
	assert.Nil(t, err)

	sche := resourceRequest.MakeScheduler()

	plans, _, err := sche(test.getNodeInfos())
	assert.Nil(t, err)

	var node = test.getNode()

	plans.ApplyChangesOnNode(node, 0)
	test.assertAfterChanges(t)

	plans.RollbackChangesOnNode(node, 0)
	test.assertAfterRollback(t)

	opts := resourcetypes.DispenseOptions{
		Node:  node,
		Index: 0,
	}
	r := &types.ResourceMeta{}
	_, err = plans.Dispense(opts, r)
	assert.Nil(t, err)
}

type requestCPUNodeTest struct {
	node      types.Node
	nodeInfos []types.NodeInfo
	cpuMap    map[string][]types.CPUMap
}

func newRequestCPUNodeTest() nodeSchdulerTest {
	return &requestCPUNodeTest{
		node: types.Node{
			Name:       "TestNode",
			CPU:        map[string]int64{"0": 10000, "1": 10000},
			NUMA:       map[string]string{"0": "0", "1": "1"},
			NUMAMemory: map[string]int64{"0": 1024, "1": 1204},
			MemCap:     10240,
		},
		nodeInfos: []types.NodeInfo{
			{
				Name:       "TestNode",
				CPUMap:     map[string]int64{"0": 10000, "1": 10000},
				NUMA:       map[string]string{"0": "0", "1": "1"},
				NUMAMemory: map[string]int64{"0": 1024, "1": 1204},
				MemCap:     10240,
				CPUPlan:    []types.CPUMap{{"0": 10000, "1": 10000}},
			},
		},
		cpuMap: map[string][]types.CPUMap{"TestNode": []types.CPUMap{{"0": 10000, "1": 10000}}},
	}
}

func (test *requestCPUNodeTest) getNodeInfos() []types.NodeInfo {
	return test.nodeInfos
}

func (test *requestCPUNodeTest) getScheduler() scheduler.Scheduler {
	mockScheduler := &schedulerMocks.Scheduler{}
	mockScheduler.On(
		"SelectCPUNodes", mock.Anything, mock.Anything, mock.Anything,
	).Return(test.nodeInfos, test.cpuMap, 1, nil)
	mockScheduler.On(
		"SelectMemoryNodess", mock.Anything, mock.Anything, mock.Anything,
	).Return(test.nodeInfos, 1, errors.New("should not select memory node here"))
	return mockScheduler
}

func (test *requestCPUNodeTest) getRequestOptions() types.ResourceOptions {
	return types.ResourceOptions{
		CPUQuotaRequest: 0.5,
		CPUQuotaLimit:   1,
		CPUBind:         true,
		MemoryRequest:   512,
		MemoryLimit:     1024,
	}
}

func (test *requestCPUNodeTest) getNode() *types.Node {
	return &test.node
}

func (test *requestCPUNodeTest) assertAfterChanges(t *testing.T) {
	assert.Less(t, test.node.CPU["0"], int64(10000))
}

func (test *requestCPUNodeTest) assertAfterRollback(t *testing.T) {
	assert.Equal(t, test.node.CPU["0"], int64(10000))
}

type requestMemNodeTest struct {
	node      types.Node
	nodeInfos []types.NodeInfo
	reqOpt    types.ResourceOptions
}

func newRequestMemNodeTest(reqOpt types.ResourceOptions) nodeSchdulerTest {
	return &requestMemNodeTest{
		node: types.Node{
			Name:       "TestNode",
			CPU:        map[string]int64{"0": 10000, "1": 10000},
			NUMA:       map[string]string{"0": "0", "1": "1"},
			NUMAMemory: map[string]int64{"0": 1024, "1": 1204},
			MemCap:     10240,
		},
		nodeInfos: []types.NodeInfo{
			{
				Name:       "TestNode",
				CPUMap:     map[string]int64{"0": 10000, "1": 10000},
				NUMA:       map[string]string{"0": "0", "1": "1"},
				NUMAMemory: map[string]int64{"0": 1024, "1": 1204},
				MemCap:     10240,
				CPUPlan:    []types.CPUMap{{"0": 10000, "1": 10000}},
			},
		},
		reqOpt: reqOpt,
	}
}

func (test *requestMemNodeTest) getRequestOptions() types.ResourceOptions {
	return test.reqOpt
}

func (test *requestMemNodeTest) getNodeInfos() []types.NodeInfo {
	return test.nodeInfos
}

func (test *requestMemNodeTest) getScheduler() scheduler.Scheduler {
	mockScheduler := &schedulerMocks.Scheduler{}
	mockScheduler.On(
		"SelectCPUNodes", mock.Anything, mock.Anything, mock.Anything,
	).Return(test.nodeInfos, nil, 1, errors.New("should not select memory node here"))
	mockScheduler.On(
		"SelectMemoryNodes", mock.Anything, mock.Anything, mock.Anything,
	).Return(test.nodeInfos, 1, nil)
	return mockScheduler
}

func (test *requestMemNodeTest) getNode() *types.Node {
	return &test.node
}

func (test *requestMemNodeTest) assertAfterChanges(t *testing.T) {
	assert.Less(t, test.node.MemCap, int64(10240))
}

func (test *requestMemNodeTest) assertAfterRollback(t *testing.T) {
	assert.Equal(t, test.node.CPU["0"], int64(10000))
}
