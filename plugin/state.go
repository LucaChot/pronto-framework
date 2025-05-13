package plugin

import (
	"sync"
)

// SignalState holds per-node reserved amounts for this cycle.
type prontoState struct {
    mu sync.Mutex
    PodReserved map[string]float64
    HostReservations map[string]float64
}


func (ps *prontoState) GetNodeReservation(nodeName string) (float64) {
    ps.mu.Lock()
    defer ps.mu.Unlock()

    return ps.HostReservations[nodeName]
}

func (ps *prontoState) ReservePod(podName, nodeName string, reserved float64) {
    ps.mu.Lock()
    defer ps.mu.Unlock()

    ps.PodReserved[podName] = reserved
    ps.HostReservations[nodeName] += reserved
}

func (ps *prontoState) UnReservePod(podName, nodeName string) {
    ps.mu.Lock()
    defer ps.mu.Unlock()

    ps.HostReservations[nodeName] -= ps.PodReserved[podName]
    delete(ps.PodReserved, podName)
}
