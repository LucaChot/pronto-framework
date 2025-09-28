package plugin

import (
	"sync"
	pb "github.com/LucaChot/pronto-framework/message"
)

// SignalState holds per-node reserved amounts for this cycle.
type prontoState struct {
    mu sync.Mutex
    PodReserved map[string]struct{}
    PodOverReserved map[string]struct{}
    HostReservations map[string]*HostInfo

    pb.UnimplementedSignalServiceServer
}


func (ps *prontoState) GetHost(nodeName string) (*HostInfo) {
    ps.mu.Lock()
    defer ps.mu.Unlock()

    if node, ok := ps.HostReservations[nodeName]; ok {
        return &HostInfo{
            Reserved: node.Reserved,
            OverReserved: node.OverReserved,
            Signal: node.Signal,
            Capacity: node.Capacity,
            Overprovision: node.Overprovision,
        }
    }
    return nil
}

func (ps *prontoState) ReservePod(podName, nodeName string, overProv bool) {
    ps.mu.Lock()
    defer ps.mu.Unlock()

    if node, ok := ps.HostReservations[nodeName]; ok {
        if !overProv {
            ps.PodReserved[podName] = struct{}{}
            node.Reserved += 1
        } else {
            ps.PodOverReserved[podName] = struct{}{}
            node.OverReserved += 1
        }
    }
}

func (ps *prontoState) UnReservePod(podName, nodeName string) {
    ps.mu.Lock()
    defer ps.mu.Unlock()

    if _, ok := ps.PodReserved[podName]; ok {
        if node, ok := ps.HostReservations[nodeName]; ok {
            node.Reserved -= 1
        }
        delete(ps.PodReserved, podName)
    }
}

func (ps *prontoState) UnOverReservePod(podName, nodeName string) {
    ps.mu.Lock()
    defer ps.mu.Unlock()

    if _, ok := ps.PodOverReserved[podName]; ok {
        if node, ok := ps.HostReservations[nodeName]; ok {
            node.OverReserved -= 1
        }
        delete(ps.PodOverReserved, podName)
    }
}

func (ps *prontoState) addNode(nodeName string) {
    ps.HostReservations[nodeName] = &HostInfo{}
}


func (ps *prontoState) AddNode(nodeName string) {
    ps.mu.Lock()
    defer ps.mu.Unlock()

    ps.addNode(nodeName)
}

func (ps *prontoState) deleteNode(nodeName string) {
    delete(ps.HostReservations, nodeName)
}

func (ps *prontoState) DeleteNode(nodeName string) {
    ps.mu.Lock()
    defer ps.mu.Unlock()

    ps.deleteNode(nodeName)
}

type HostOptions = func(*HostInfo)

func WithSignal(signal float64) HostOptions {
    return func(hi *HostInfo) {
        hi.Signal = signal
    }
}

func WithCapacity(capacity float64) HostOptions {
    return func(hi *HostInfo) {
        hi.Capacity = capacity
    }
}

func WithOverprovision(overProvision float64) HostOptions {
    return func(hi *HostInfo) {
        hi.Overprovision = overProvision
    }
}

func (ps *prontoState) UpdateHostInfo(name string, opts ...HostOptions) {
    ps.mu.Lock()
    defer ps.mu.Unlock()

    node, ok := ps.HostReservations[name]
    if !ok {
        ps.addNode(name)
        node = ps.HostReservations[name]
    }

    for _, opt := range opts {
        opt(node)
    }
}






