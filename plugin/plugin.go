package plugin

import (
	"context"
	"fmt"
	"math"
	//"strconv"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"
	framework "k8s.io/kubernetes/pkg/scheduler/framework"
)

const Name = "Pronto"

type HostInfo struct {
    Reserved        int
    OverReserved    int
	Signal          float64
	Capacity        float64
	Overprovision   float64
}

type BooleanState struct {
    val bool
}

// Clone is required so CycleState can copy your data safely
func (b *BooleanState) Clone() framework.StateData {
    return &BooleanState{ val: b.val }
}

// ProntoPlugin implements a Score, Reserve, Unreserve plugin
// that tracks a per-node signal using a Kalman filter and CycleState.
type ProntoPlugin struct {
    logger klog.Logger
    handle      framework.Handle

    prontoState
}

var _ framework.FilterPlugin = &ProntoPlugin{}
var _ framework.ScorePlugin = &ProntoPlugin{}
var _ framework.ReservePlugin = &ProntoPlugin{}

// New initializes a new plugin and returns it.
func New(ctx context.Context, obj runtime.Object, handle framework.Handle) (framework.Plugin, error) {
	logger := klog.FromContext(ctx).WithValues("plugin", Name)
	pl := &ProntoPlugin{logger: logger, handle: handle}
	pl.HostReservations = make(map[string]*HostInfo)
	pl.PodReserved = make(map[string]struct{})
	pl.PodOverReserved = make(map[string]struct{})

	podInformer := handle.SharedInformerFactory().Core().V1().Pods()
	podInformer.Informer().AddEventHandler(
		cache.ResourceEventHandlerFuncs{
			UpdateFunc: pl.onPodUpdate,
            DeleteFunc: pl.onPodDelete,
		},
	)

    pl.prontoState.startPlacementServer(ctx, logger)

	return pl, nil
}


// Name returns the plugin name.
func (pl *ProntoPlugin) Name() string { return Name }

func (pl *ProntoPlugin) Filter(ctx context.Context, state *framework.CycleState, pod *v1.Pod, nodeInfo *framework.NodeInfo) *framework.Status {
    logger := klog.FromContext(klog.NewContext(ctx, pl.logger)).WithValues("ExtensionPoint", "Filter")
	node := nodeInfo.Node()
	if node == nil {
		return framework.NewStatus(framework.Error, "node not found")
	}

    hostInfo := pl.GetHost(node.Name)
    if hostInfo == nil {
        return framework.NewStatus(framework.Unschedulable,
            fmt.Sprintf("Node %v does not exist", node.Name))
    }

    needed := 1e-3

    if logger.V(10).Enabled() {
        logger.Info("Pronto Signal", "Node Name", node.Name, "HostInfo", hostInfo,
            "needed", needed)
    }

    if hostInfo.Capacity - float64(hostInfo.Reserved) > 1 {
        state.Write(framework.StateKey(node.Name), &BooleanState{val: false})
        return framework.NewStatus(framework.Success, "")
    }

    //if hostInfo.Overprovision - float64(hostInfo.OverReserved) > needed {
        //state.Write(framework.StateKey(node.Name), &BooleanState{val: true})
        //return framework.NewStatus(framework.Success, "")
    //}

    return framework.NewStatus(framework.Unschedulable,
        fmt.Sprintf("Node %v does not meet signal requirements: capacity: %f reserved: %d", node.Name, hostInfo.Capacity, hostInfo.Reserved))
}

// Score reads the node signal, predicts via Kalman, subtracts reserved, and returns a score.
func (pl *ProntoPlugin) Score(ctx context.Context, state *framework.CycleState, pod *v1.Pod, nodeName string,) (int64, *framework.Status) {
    logger := klog.FromContext(klog.NewContext(ctx, pl.logger)).WithValues("ExtensionPoint", "Score")
    nodeInfo, err := pl.handle.SnapshotSharedLister().NodeInfos().Get(nodeName)
    if err != nil {
        return 0, framework.NewStatus(framework.Error, fmt.Sprintf("getting node %q from Snapshot: %v", nodeName, err))
    }

	node := nodeInfo.Node()
	if node == nil {
		return 0, framework.NewStatus(framework.Error, "node not found")
	}

    hostInfo := pl.GetHost(node.Name)
    score := signalScorer(hostInfo.Capacity)

    if logger.V(10).Enabled() {
        logger.Info("Pronto Signal", "podName", pod.Name, "nodeName", node.Name, "scorer", Name,
            "signal", hostInfo.Signal, "score", score)
    }

    return int64(score), nil
}

func signalScorer(signal float64) int64 {
    return int64(signal * 100)
}

// extractSignalFromNode reads a numeric signal from a Node label.
//func extractSignalFromNode(node *v1.Node) float64 {
    //if val, ok := node.Annotations["pronto/signal"]; ok {
        //if f, err := strconv.ParseFloat(val, 64); err == nil {
            //return f
        //}
    //}
    //return 0
//}
//
// extractSignalFromNode reads a numeric signal from a Node label.
//func extractCapacityFromNode(node *v1.Node) float64 {
    //if val, ok := node.Annotations["pronto/pod-cost"]; ok {
        //if f, err := strconv.ParseFloat(val, 64); err == nil {
            //return f
        //}
    //}
    //return 0.1
//}
//
func (pl *ProntoPlugin) ScoreExtensions() framework.ScoreExtensions {
    return pl
}

// NormalizeScore invoked after scoring all nodes.
func (pl *ProntoPlugin) NormalizeScore(ctx context.Context, state *framework.CycleState, pod *v1.Pod, scores framework.NodeScoreList) *framework.Status {
	// Find highest and lowest scores.
	var highest int64 = -math.MaxInt64
	var lowest int64 = math.MaxInt64
	for _, nodeScore := range scores {
		if nodeScore.Score > highest {
			highest = nodeScore.Score
		}
		if nodeScore.Score < lowest {
			lowest = nodeScore.Score
		}
	}

	// Transform the highest to lowest score range to fit the framework's min to max node score range.
	oldRange := highest - lowest
	newRange := framework.MaxNodeScore - framework.MinNodeScore
	for i, nodeScore := range scores {
		if oldRange == 0 {
			scores[i].Score = framework.MinNodeScore
		} else {
			scores[i].Score = ((nodeScore.Score - lowest) * newRange / oldRange) + framework.MinNodeScore
		}
	}

	return nil
}


// Reserve updates the Kalman filter with a measurement and records reserved amount.
func (pl *ProntoPlugin) Reserve(
    ctx context.Context,
    state *framework.CycleState,
    pod *v1.Pod,
    nodeName string,
) *framework.Status {
    logger := klog.FromContext(klog.NewContext(ctx, pl.logger)).WithValues("ExtensionPoint", "Reserve")
    nodeInfo, err := pl.handle.SnapshotSharedLister().NodeInfos().Get(nodeName)
    if err != nil {
        return framework.NewStatus(framework.Error, fmt.Sprintf("getting node %q from Snapshot: %v", nodeName, err))
    }

	node := nodeInfo.Node()
	if node == nil {
		return framework.NewStatus(framework.Error, "node not found")
	}

    oversat, err := state.Read(framework.StateKey(nodeName))
    if err != nil {
        return framework.NewStatus(framework.Error, "node info missing")
    }

    if logger.V(10).Enabled() {
        logger.Info("Pronto Signal", "Node Name", node.Name, "OverSaturated", oversat.(*BooleanState).val)
    }

    //pl.ReservePod(pod.Name, nodeName, oversat.(*BooleanState).val)
    pl.ReservePod(pod.Name, nodeName, false)

    return framework.NewStatus(framework.Success, "")
}

// Unreserve subtracts the reserved amount if scheduling fails.
func (pl *ProntoPlugin) Unreserve(
    ctx context.Context,
    state *framework.CycleState,
    pod *v1.Pod,
    nodeName string,
) {
    pl.UnReservePod(pod.Name, nodeName)
    pl.UnOverReservePod(pod.Name, nodeName)
}

func (pl *ProntoPlugin) onPodUpdate(oldObj, newObj interface{}) {
    oldPod := oldObj.(*v1.Pod)
    newPod := newObj.(*v1.Pod)

    //oldRunning := oldPod.Status.Phase == v1.PodPending
    newPending := newPod.Status.Phase == v1.PodPending

    // Only consider pods that have a node assigned
    nodeName := newPod.Spec.NodeName

    // Entered Running
    if !newPending && nodeName != "" {
        pl.UnReservePod(oldPod.Name, nodeName)
    }
}
func (pl *ProntoPlugin) onPodDelete(obj interface{}) {
    pod := obj.(*v1.Pod)
    // Only consider pods that have a node assigned
    nodeName := pod.Spec.NodeName

    pl.UnReservePod(pod.Name, nodeName)
    pl.UnOverReservePod(pod.Name, nodeName)
}
